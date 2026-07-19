// Package vault implements a store.Plugin backed by HashiCorp Vault's KV v2
// secrets engine. Each configuration value is stored as a separate Vault
// secret, enabling native value-level versioning, encryption at rest (via
// Vault's storage barrier), authentication, and access control through Vault
// ACL policies.
//
// Storage layout (with default mount="secret", prefix="zhi"):
//
//	secret/data/zhi/{tree_id}/{path}      — value data
//	secret/metadata/zhi/{tree_id}/{path}  — value metadata and versions
//
// For example, the config path "db/host" in tree "prod" maps to the Vault
// secret at path "zhi/prod/db/host" within the "secret" KV v2 engine.
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MrWong99/zhi/internal/tlsutil"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// Config holds Vault connection and storage settings.
type Config struct {
	// Address is the Vault server address.
	// Defaults to the VAULT_ADDR environment variable, or "http://127.0.0.1:8200".
	Address string

	// Token is the initial Vault authentication token.
	// Defaults to the VAULT_TOKEN environment variable.
	Token string

	// Mount is the KV v2 secrets engine mount point.
	// Defaults to "secret".
	Mount string

	// Prefix is the key prefix within the mount for all zhi data.
	// Defaults to "zhi".
	Prefix string

	// Namespace is the Vault namespace (enterprise feature).
	// Defaults to the VAULT_NAMESPACE environment variable.
	Namespace string

	// CACert is the path to a PEM-encoded CA certificate for verifying
	// the Vault server's TLS certificate.
	// Defaults to the VAULT_CACERT environment variable.
	CACert string

	// ClientCert is the path to a PEM-encoded client certificate for
	// mutual TLS authentication with Vault.
	// Defaults to the VAULT_CLIENT_CERT environment variable.
	ClientCert string

	// ClientKey is the path to a PEM-encoded client private key for
	// mutual TLS authentication with Vault.
	// Defaults to the VAULT_CLIENT_KEY environment variable.
	ClientKey string

	// SkipVerify disables TLS certificate verification. This should
	// only be used for development and testing.
	// Defaults to the VAULT_SKIP_VERIFY environment variable.
	SkipVerify bool
}

// DefaultConfig returns a Config populated from environment variables
// with sensible defaults.
func DefaultConfig() Config {
	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:8200"
	}
	skipVerify := os.Getenv("VAULT_SKIP_VERIFY")
	return Config{
		Address:    addr,
		Token:      os.Getenv("VAULT_TOKEN"),
		Mount:      "secret",
		Prefix:     "zhi",
		Namespace:  os.Getenv("VAULT_NAMESPACE"),
		CACert:     os.Getenv("VAULT_CACERT"),
		ClientCert: os.Getenv("VAULT_CLIENT_CERT"),
		ClientKey:  os.Getenv("VAULT_CLIENT_KEY"),
		SkipVerify: skipVerify == "true" || skipVerify == "1",
	}
}

// Store implements store.Plugin using Vault's KV v2 secrets engine.
type Store struct {
	client *vaultClient
	mount  string
	prefix string

	renewMu   sync.Mutex
	stopRenew chan struct{}
}

// New creates a new Vault store with the given configuration.
func New(cfg Config) (*Store, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("vault address is required")
	}
	if cfg.Mount == "" {
		cfg.Mount = "secret"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "zhi"
	}

	httpClient, err := buildHTTPClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("configuring vault TLS: %w", err)
	}

	s := &Store{
		client: newVaultClient(cfg.Address, cfg.Token, cfg.Namespace, httpClient),
		mount:  cfg.Mount,
		prefix: cfg.Prefix,
	}
	return s, nil
}

// buildHTTPClient constructs an [http.Client] with TLS settings from
// the Config. When TLS fields (CACert, ClientCert, ClientKey, SkipVerify)
// are set, it builds a custom transport using [tlsutil.ClientConfig].
// Otherwise, it returns a plain client with a default timeout.
func buildHTTPClient(cfg Config) (*http.Client, error) {
	tlsCfg := tlsutil.ClientConfig{
		CAFile:             cfg.CACert,
		CertFile:           cfg.ClientCert,
		KeyFile:            cfg.ClientKey,
		InsecureSkipVerify: cfg.SkipVerify,
	}

	if tlsCfg.Enabled() {
		if cfg.SkipVerify {
			slog.Warn("VAULT_SKIP_VERIFY is enabled, TLS certificate verification disabled")
		}
		return tlsCfg.HTTPClient(30 * time.Second)
	}

	return &http.Client{Timeout: 30 * time.Second}, nil
}

// Stop cancels background token renewal. It is safe to call multiple times.
func (s *Store) Stop() {
	s.renewMu.Lock()
	defer s.renewMu.Unlock()
	if s.stopRenew != nil {
		close(s.stopRenew)
		s.stopRenew = nil
	}
}

// --- Path helpers ---

// secretPath returns the Vault secret path for a tree value.
// Example: "zhi/prod/db/host"
func (s *Store) secretPath(treeID, path string) string {
	return s.prefix + "/" + treeID + "/" + path
}

// dataPath returns the full KV v2 data API path.
func (s *Store) dataPath(treeID, path string) string {
	return s.mount + "/data/" + s.secretPath(treeID, path)
}

// metadataPath returns the full KV v2 metadata API path.
func (s *Store) metadataPath(treeID, path string) string {
	return s.mount + "/metadata/" + s.secretPath(treeID, path)
}

// destroyPath returns the full KV v2 destroy API path.
func (s *Store) destroyPath(treeID, path string) string {
	return s.mount + "/destroy/" + s.secretPath(treeID, path)
}

// treeMetadataPrefix returns the metadata LIST path for a tree's values.
func (s *Store) treeMetadataPrefix(treeID string) string {
	return s.mount + "/metadata/" + s.prefix + "/" + treeID + "/"
}

// treesMetadataPrefix returns the metadata LIST path for all trees.
func (s *Store) treesMetadataPrefix() string {
	return s.mount + "/metadata/" + s.prefix + "/"
}

// --- Capabilities ---

func (s *Store) Capabilities(_ context.Context) (*store.Capabilities, error) {
	return &store.Capabilities{
		Versioning:    store.VersioningValue,
		Encryption:    store.EncryptionActive,
		Auth:          true,
		AccessControl: true,
	}, nil
}

// --- Authentication ---

func (s *Store) AuthMethods(_ context.Context) ([]store.AuthMethod, error) {
	return []store.AuthMethod{
		{
			Type:        "token",
			Description: "Authenticate with a Vault token directly",
			Fields: []store.AuthField{
				{Name: "token", Description: "Vault token", Required: true, Secret: true},
			},
		},
		{
			Type:        "userpass",
			Description: "Authenticate with username and password",
			Fields: []store.AuthField{
				{Name: "username", Description: "Username", Required: true, Secret: false},
				{Name: "password", Description: "Password", Required: true, Secret: true},
			},
		},
		{
			Type:        "approle",
			Description: "Authenticate with AppRole credentials",
			Fields: []store.AuthField{
				{Name: "role_id", Description: "Role ID", Required: true, Secret: false},
				{Name: "secret_id", Description: "Secret ID", Required: true, Secret: true},
			},
		},
		{
			Type:        "ldap",
			Description: "Authenticate via LDAP directory",
			Fields: []store.AuthField{
				{Name: "username", Description: "LDAP username", Required: true, Secret: false},
				{Name: "password", Description: "LDAP password", Required: true, Secret: true},
			},
		},
		{
			Type:        "kubernetes",
			Description: "Authenticate using a Kubernetes service account token",
			Fields: []store.AuthField{
				{Name: "role", Description: "Vault role name", Required: true, Secret: false},
				{Name: "jwt", Description: "Service account JWT", Required: true, Secret: true},
			},
		},
		{
			Type:        "oidc",
			Description: "Authenticate via OIDC provider",
			Interactive: true,
			Fields: []store.AuthField{
				{Name: "role", Description: "OIDC role name", Required: false, Secret: false},
			},
		},
	}, nil
}

func (s *Store) Login(ctx context.Context, method string, credentials map[string]string) (*store.Credential, error) {
	switch method {
	case "token":
		return s.loginToken(ctx, credentials)
	case "userpass":
		return s.loginUserpass(ctx, credentials)
	case "approle":
		return s.loginAppRole(ctx, credentials)
	case "ldap":
		return s.loginLDAP(ctx, credentials)
	case "kubernetes":
		return s.loginKubernetes(ctx, credentials)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", method)
	}
}

func (s *Store) loginToken(ctx context.Context, creds map[string]string) (*store.Credential, error) {
	token := creds["token"]
	if token == "" {
		return nil, &store.ErrAuthRequired{Reason: "token is required"}
	}
	s.client.setToken(token)

	// Validate by looking up the token.
	resp, err := s.client.request(ctx, http.MethodGet, "auth/token/lookup-self", nil)
	if err != nil {
		s.client.setToken("")
		return nil, s.mapAuthError(err)
	}

	cred := &store.Credential{
		Token:    token,
		Metadata: make(map[string]string),
	}
	if resp.Data != nil {
		var lookup struct {
			ExpireTime string `json:"expire_time"`
			TTL        int    `json:"ttl"`
			Renewable  bool   `json:"renewable"`
		}
		if json.Unmarshal(resp.Data, &lookup) == nil {
			if lookup.ExpireTime != "" {
				cred.ExpiresAt = lookup.ExpireTime
			}
			if lookup.Renewable && lookup.TTL > 0 {
				s.startRenewal(ctx, lookup.TTL)
			}
		}
	}
	return cred, nil
}

func (s *Store) loginUserpass(ctx context.Context, creds map[string]string) (*store.Credential, error) {
	username := creds["username"]
	password := creds["password"]
	if username == "" || password == "" {
		return nil, &store.ErrAuthRequired{Reason: "username and password are required"}
	}

	resp, err := s.client.request(ctx, http.MethodPost, "auth/userpass/login/"+username, map[string]any{
		"password": password,
	})
	if err != nil {
		return nil, s.mapAuthError(err)
	}
	return s.processAuthResponse(ctx, resp)
}

func (s *Store) loginAppRole(ctx context.Context, creds map[string]string) (*store.Credential, error) {
	roleID := creds["role_id"]
	secretID := creds["secret_id"]
	if roleID == "" || secretID == "" {
		return nil, &store.ErrAuthRequired{Reason: "role_id and secret_id are required"}
	}

	resp, err := s.client.request(ctx, http.MethodPost, "auth/approle/login", map[string]any{
		"role_id":   roleID,
		"secret_id": secretID,
	})
	if err != nil {
		return nil, s.mapAuthError(err)
	}
	return s.processAuthResponse(ctx, resp)
}

func (s *Store) loginLDAP(ctx context.Context, creds map[string]string) (*store.Credential, error) {
	username := creds["username"]
	password := creds["password"]
	if username == "" || password == "" {
		return nil, &store.ErrAuthRequired{Reason: "username and password are required"}
	}

	resp, err := s.client.request(ctx, http.MethodPost, "auth/ldap/login/"+username, map[string]any{
		"password": password,
	})
	if err != nil {
		return nil, s.mapAuthError(err)
	}
	return s.processAuthResponse(ctx, resp)
}

func (s *Store) loginKubernetes(ctx context.Context, creds map[string]string) (*store.Credential, error) {
	role := creds["role"]
	jwt := creds["jwt"]
	if role == "" || jwt == "" {
		return nil, &store.ErrAuthRequired{Reason: "role and jwt are required"}
	}

	resp, err := s.client.request(ctx, http.MethodPost, "auth/kubernetes/login", map[string]any{
		"role": role,
		"jwt":  jwt,
	})
	if err != nil {
		return nil, s.mapAuthError(err)
	}
	return s.processAuthResponse(ctx, resp)
}

// LoginInteractive starts an OIDC browser-based authentication flow.
// It contacts Vault's OIDC auth endpoint to obtain an authorization URL
// that the user must visit in their browser.
func (s *Store) LoginInteractive(ctx context.Context, method string, params map[string]string) (*store.InteractiveChallenge, error) {
	if method != "oidc" {
		return nil, fmt.Errorf("interactive login not supported for method %q", method)
	}

	body := map[string]any{}
	if role := params["role"]; role != "" {
		body["role"] = role
	}
	if redirectURI := params["redirect_uri"]; redirectURI != "" {
		body["redirect_uri"] = redirectURI
	}

	resp, err := s.client.request(ctx, http.MethodPost, "auth/oidc/oidc/auth_url", body)
	if err != nil {
		return nil, s.mapAuthError(err)
	}

	var authData struct {
		AuthURL string `json:"auth_url"`
	}
	if err := json.Unmarshal(resp.Data, &authData); err != nil {
		return nil, fmt.Errorf("parsing OIDC auth_url response: %w", err)
	}
	if authData.AuthURL == "" {
		return nil, fmt.Errorf("vault returned empty OIDC auth_url")
	}

	// Extract state from the auth URL to use as challenge ID.
	challengeID := extractQueryParam(authData.AuthURL, "state")
	if challengeID == "" {
		challengeID = authData.AuthURL // fallback
	}

	return &store.InteractiveChallenge{
		ChallengeID: challengeID,
		AuthURL:     authData.AuthURL,
		ExpiresAt:   time.Now().Add(5 * time.Minute).Format(time.RFC3339),
	}, nil
}

// LoginInteractiveCallback completes the OIDC authentication flow by
// exchanging the callback parameters (code, state) with Vault.
func (s *Store) LoginInteractiveCallback(ctx context.Context, _ string, callbackParams map[string]string) (*store.Credential, error) {
	body := make(map[string]any, len(callbackParams))
	for k, v := range callbackParams {
		body[k] = v
	}

	resp, err := s.client.request(ctx, http.MethodPut, "auth/oidc/oidc/callback", body)
	if err != nil {
		return nil, s.mapAuthError(err)
	}
	return s.processAuthResponse(ctx, resp)
}

// extractQueryParam extracts a query parameter from a URL string.
func extractQueryParam(rawURL, param string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(param)
}

// processAuthResponse extracts the token from a login response, stores it,
// starts renewal if applicable, and returns a Credential.
func (s *Store) processAuthResponse(ctx context.Context, resp *vaultResponse) (*store.Credential, error) {
	if resp.Auth == nil {
		return nil, &store.ErrAuthRequired{Reason: "no auth data in response"}
	}

	s.client.setToken(resp.Auth.ClientToken)

	cred := &store.Credential{
		Token:    resp.Auth.ClientToken,
		Metadata: make(map[string]string),
	}
	maps.Copy(cred.Metadata, resp.Auth.Metadata)
	if resp.Auth.LeaseDuration > 0 {
		expiry := time.Now().Add(time.Duration(resp.Auth.LeaseDuration) * time.Second)
		cred.ExpiresAt = expiry.Format(time.RFC3339)
	}

	if resp.Auth.Renewable && resp.Auth.LeaseDuration > 0 {
		s.startRenewal(ctx, resp.Auth.LeaseDuration)
	}

	return cred, nil
}

// startRenewal starts a background goroutine that renews the Vault token
// at half the lease duration interval. The goroutine exits when Stop() is
// called or renewal fails.
//
// The renewal loop is deliberately detached from the caller's request-scoped
// context (via context.WithoutCancel): the parent passed to Login is often a
// per-request context (an HTTP request in the web UI, or the unary RPC context
// when the store runs as an external plugin) that is cancelled the instant the
// login call returns. Tying renewal to it would kill the goroutine before the
// first renewal ever fires, silently letting the token expire at lease end.
func (s *Store) startRenewal(parent context.Context, leaseDuration int) {
	s.renewMu.Lock()
	defer s.renewMu.Unlock()

	// Detach from the request-scoped context so renewal outlives the login call.
	renewCtx := context.WithoutCancel(parent)

	// Stop any existing renewal.
	if s.stopRenew != nil {
		close(s.stopRenew)
	}

	stop := make(chan struct{})
	s.stopRenew = stop

	renewInterval := max(time.Duration(leaseDuration)*time.Second/2, time.Second)

	go func() {
		timer := time.NewTimer(renewInterval)
		defer timer.Stop()
		for {
			select {
			case <-stop:
				return
			case <-timer.C:
				ctx, cancel := context.WithTimeout(renewCtx, 30*time.Second)
				_, err := s.client.request(ctx, http.MethodPost, "auth/token/renew-self", nil)
				cancel()
				if err != nil {
					slog.Warn("vault token renewal failed, token will expire", "error", err)
					return
				}
				timer.Reset(renewInterval)
			}
		}
	}()
}

// mapAuthError converts a Vault API error to a store error type.
func (s *Store) mapAuthError(err error) error {
	var apiErr *vaultAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusForbidden:
			return &store.ErrAccessDenied{Action: "login"}
		case http.StatusBadRequest, http.StatusUnauthorized:
			return &store.ErrAuthRequired{Reason: apiErr.Message}
		}
	}
	return err
}

// --- Tree management ---

func (s *Store) ListTrees(ctx context.Context) ([]string, error) {
	resp, err := s.client.list(ctx, s.treesMetadataPrefix())
	if err != nil {
		if isVaultNotFound(err) {
			return nil, nil
		}
		return nil, s.mapError(err)
	}

	keys, err := parseListKeys(resp)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		// Vault LIST returns "dir/" for subdirectories.
		ids = append(ids, strings.TrimSuffix(k, "/"))
	}
	return ids, nil
}

func (s *Store) DeleteTree(ctx context.Context, id string) error {
	// Recursively list all value paths under this tree and delete their metadata.
	paths, err := s.listAllPaths(ctx, s.treeMetadataPrefix(id), "")
	if err != nil {
		return err
	}
	for _, p := range paths {
		_, delErr := s.client.request(ctx, http.MethodDelete, s.metadataPath(id, p), nil)
		if delErr != nil && !isVaultNotFound(delErr) {
			return s.mapError(delErr)
		}
	}
	return nil
}

// --- Value operations ---

func (s *Store) GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error) {
	result := make(map[string]config.Value, len(paths))
	for _, p := range paths {
		val, found, err := s.getValue(ctx, id, p, 0)
		if err != nil {
			return nil, err
		}
		if found {
			// store.writeonly: return metadata but redact the actual value.
			if labels.IsWriteonly(val.Metadata) {
				val.Val = nil
			}
			result[p] = val
		}
	}
	return result, nil
}

func (s *Store) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
	for p, v := range values {
		// Apply per-value metadata settings to Vault KV v2 metadata endpoint
		// before writing the data. This configures TTL, max versions, etc.
		if err := s.applyValueMetadata(ctx, id, p, v.Metadata); err != nil {
			return err
		}

		encoded, err := encodeValue(v)
		if err != nil {
			return fmt.Errorf("encoding value for path %q: %w", p, err)
		}

		body := map[string]any{
			"data": encoded,
		}

		// Wire CAS: explicit PutOptions take precedence, then fall back
		// to Value.Version (populated by prior reads for optimistic locking).
		// -1 is the "no CAS requested" sentinel so that an explicit cas=0
		// (Vault KV v2 "write only if the secret does not exist") is honored
		// rather than being conflated with "unset".
		casVersion := -1
		if opts != nil && opts.CASVersions != nil {
			if expected, ok := opts.CASVersions[p]; ok {
				cas, err := strconv.Atoi(expected)
				if err != nil {
					return fmt.Errorf("invalid CAS version %q for path %q: %w", expected, p, err)
				}
				casVersion = cas
			}
		}
		if casVersion < 0 && v.Version != "" {
			cas, err := strconv.Atoi(v.Version)
			if err == nil {
				casVersion = cas
			}
		}
		if casVersion >= 0 {
			body["options"] = map[string]any{"cas": casVersion}
		}

		_, err = s.client.request(ctx, http.MethodPost, s.dataPath(id, p), body)
		if err != nil {
			return s.mapWriteError(err, p)
		}
	}
	return nil
}

// applyValueMetadata configures Vault KV v2 per-secret metadata based on
// store labels. This uses the metadata API endpoint to set properties like
// max_versions and delete_version_after on individual secrets.
func (s *Store) applyValueMetadata(ctx context.Context, treeID, path string, metadata map[string]any) error {
	metaBody := make(map[string]any)

	// store.noversion: limit to 1 version so no history is kept.
	if labels.IsNoVersion(metadata) {
		metaBody["max_versions"] = 1
	}

	// store.maxversions: set the maximum number of versions to retain.
	// If store.noversion is also set, noversion takes precedence (1 version).
	if maxV := labels.GetMaxVersions(metadata); maxV > 0 && !labels.IsNoVersion(metadata) {
		metaBody["max_versions"] = maxV
	}

	// store.ttl: set delete_version_after to automatically expire versions.
	// Vault KV v2 accepts a Go-style duration string.
	if ttl := labels.GetTTL(metadata); ttl > 0 {
		metaBody["delete_version_after"] = fmt.Sprintf("%ds", ttl)
	}

	if len(metaBody) == 0 {
		return nil
	}

	_, err := s.client.request(ctx, http.MethodPost, s.metadataPath(treeID, path), metaBody)
	if err != nil {
		return s.mapError(err)
	}
	return nil
}

func (s *Store) DeleteValues(ctx context.Context, id string, paths []string) error {
	for _, p := range paths {
		// Delete metadata permanently removes all versions.
		_, err := s.client.request(ctx, http.MethodDelete, s.metadataPath(id, p), nil)
		if err != nil && !isVaultNotFound(err) {
			return s.mapError(err)
		}
	}
	return nil
}

// --- Tree-level versioning (not supported — Vault KV v2 is value-level) ---

func (s *Store) ListTreeVersions(_ context.Context, _ string) ([]string, error) {
	return nil, &store.ErrNotSupported{Feature: "tree versioning"}
}

func (s *Store) GetTreeVersion(_ context.Context, _ string, _ string, _ []string) (map[string]config.Value, error) {
	return nil, &store.ErrNotSupported{Feature: "tree versioning"}
}

func (s *Store) RollbackTree(_ context.Context, _ string, _ string) error {
	return &store.ErrNotSupported{Feature: "tree versioning"}
}

func (s *Store) DeleteTreeVersion(_ context.Context, _ string, _ string) error {
	return &store.ErrNotSupported{Feature: "tree versioning"}
}

// --- Value-level versioning ---

func (s *Store) ListValueVersions(ctx context.Context, id string, path string) ([]string, error) {
	resp, err := s.client.request(ctx, http.MethodGet, s.metadataPath(id, path), nil)
	if err != nil {
		if isVaultNotFound(err) {
			return nil, nil
		}
		return nil, s.mapError(err)
	}

	var meta kvMetadata
	if err := json.Unmarshal(resp.Data, &meta); err != nil {
		return nil, fmt.Errorf("parsing metadata: %w", err)
	}

	// Collect non-destroyed version numbers and sort newest first.
	var versions []int
	for vStr, vm := range meta.Versions {
		if vm.Destroyed {
			continue
		}
		v, err := strconv.Atoi(vStr)
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))

	result := make([]string, len(versions))
	for i, v := range versions {
		result[i] = strconv.Itoa(v)
	}
	return result, nil
}

func (s *Store) GetValueVersion(ctx context.Context, id string, path string, version string) (config.Value, bool, error) {
	v, err := strconv.Atoi(version)
	if err != nil {
		return config.Value{}, false, fmt.Errorf("invalid version %q: %w", version, err)
	}

	val, found, err := s.getValue(ctx, id, path, v)
	if err != nil {
		return config.Value{}, false, err
	}
	return val, found, nil
}

func (s *Store) RollbackValue(ctx context.Context, id string, path string, version string) error {
	// Read the target version.
	val, found, err := s.GetValueVersion(ctx, id, path, version)
	if err != nil {
		return err
	}
	if !found {
		return &store.ErrPathNotFound{TreeID: id, Path: path}
	}

	// Write it as a new version.
	encoded, err := encodeValue(val)
	if err != nil {
		return fmt.Errorf("encoding value for rollback of %q: %w", path, err)
	}
	body := map[string]any{
		"data": encoded,
	}
	_, err = s.client.request(ctx, http.MethodPost, s.dataPath(id, path), body)
	if err != nil {
		return s.mapError(err)
	}
	return nil
}

func (s *Store) DeleteValueVersion(ctx context.Context, id string, path string, version string) error {
	v, err := strconv.Atoi(version)
	if err != nil {
		return fmt.Errorf("invalid version %q: %w", version, err)
	}

	// Permanently destroy the specific version.
	_, err = s.client.request(ctx, http.MethodPost, s.destroyPath(id, path), map[string]any{
		"versions": []int{v},
	})
	if err != nil {
		return s.mapError(err)
	}
	return nil
}

// --- Encryption ---

// Vault handles encryption transparently via its storage barrier.
// InitEncryption is a no-op since encryption is always active.
func (s *Store) InitEncryption(_ context.Context, _ []byte) error {
	return nil
}

// RotateEncryption is a no-op since Vault manages its own barrier key rotation.
func (s *Store) RotateEncryption(_ context.Context, _, _ []byte) error {
	return nil
}

// --- Access control via Vault ACL policies ---

// policyName returns the Vault ACL policy name for a tree/user combination.
func policyName(treeID, user string) string {
	return "zhi-" + treeID + "-" + user
}

func (s *Store) GrantAccess(ctx context.Context, id string, user string, permissions []store.Permission) error {
	name := policyName(id, user)

	// Read existing policy (if any) and merge with new permissions.
	existingPerms := s.readPolicyPermissions(ctx, name, id)
	merged := mergePermissions(existingPerms, permissions)

	policy := generatePolicy(s.mount, s.prefix, id, merged)
	_, err := s.client.request(ctx, http.MethodPut, "sys/policies/acl/"+name, map[string]any{
		"policy": policy,
	})
	if err != nil {
		return s.mapError(err)
	}
	return nil
}

func (s *Store) RevokeAccess(ctx context.Context, id string, user string, paths []string) error {
	name := policyName(id, user)

	if len(paths) == 0 {
		// Revoke all access by deleting the policy.
		_, err := s.client.request(ctx, http.MethodDelete, "sys/policies/acl/"+name, nil)
		if err != nil && !isVaultNotFound(err) {
			return s.mapError(err)
		}
		return nil
	}

	// Read existing policy, remove specified paths, and rewrite.
	existingPerms := s.readPolicyPermissions(ctx, name, id)
	filtered := removePermissionPaths(existingPerms, paths)

	if len(filtered) == 0 {
		// No permissions left; delete the policy.
		_, _ = s.client.request(ctx, http.MethodDelete, "sys/policies/acl/"+name, nil)
		return nil
	}

	policy := generatePolicy(s.mount, s.prefix, id, filtered)
	_, err := s.client.request(ctx, http.MethodPut, "sys/policies/acl/"+name, map[string]any{
		"policy": policy,
	})
	if err != nil {
		return s.mapError(err)
	}
	return nil
}

func (s *Store) ListAccess(ctx context.Context, id string) (map[string][]store.Permission, error) {
	// List all policies matching our naming convention.
	resp, err := s.client.list(ctx, "sys/policies/acl")
	if err != nil {
		if isVaultNotFound(err) {
			return nil, nil
		}
		return nil, s.mapError(err)
	}

	keys, err := parseListKeys(resp)
	if err != nil {
		return nil, err
	}

	prefix := "zhi-" + id + "-"
	result := make(map[string][]store.Permission)

	for _, k := range keys {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		user := strings.TrimPrefix(k, prefix)
		perms := s.readPolicyPermissions(ctx, k, id)
		if len(perms) > 0 {
			result[user] = perms
		}
	}
	return result, nil
}

// --- Internal helpers ---

// getValue reads a single value from Vault KV v2. If version is 0, reads
// the latest version. Returns (value, found, error).
func (s *Store) getValue(ctx context.Context, treeID, path string, version int) (config.Value, bool, error) {
	apiPath := s.dataPath(treeID, path)
	if version > 0 {
		apiPath += "?version=" + strconv.Itoa(version)
	}

	resp, err := s.client.request(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		if isVaultNotFound(err) {
			return config.Value{}, false, nil
		}
		return config.Value{}, false, s.mapError(err)
	}

	val, err := decodeKVResponse(resp)
	if err != nil {
		return config.Value{}, false, err
	}
	return val, true, nil
}

// listAllPaths recursively lists all secret paths under a Vault metadata prefix.
func (s *Store) listAllPaths(ctx context.Context, basePath, relPrefix string) ([]string, error) {
	resp, err := s.client.list(ctx, basePath)
	if err != nil {
		if isVaultNotFound(err) {
			return nil, nil
		}
		return nil, s.mapError(err)
	}

	keys, err := parseListKeys(resp)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			// Subdirectory: recurse.
			sub, err := s.listAllPaths(ctx, basePath+key, relPrefix+key)
			if err != nil {
				return nil, err
			}
			result = append(result, sub...)
		} else {
			result = append(result, relPrefix+key)
		}
	}
	return result, nil
}

// mapError converts Vault API errors to store error types.
func (s *Store) mapError(err error) error {
	var apiErr *vaultAPIError
	if !errors.As(err, &apiErr) {
		return err
	}
	switch apiErr.StatusCode {
	case http.StatusForbidden:
		return &store.ErrAccessDenied{Action: apiErr.Message}
	case http.StatusUnauthorized:
		return &store.ErrAuthRequired{Reason: apiErr.Message}
	case http.StatusNotFound:
		return &store.ErrPathNotFound{}
	default:
		return err
	}
}

// mapWriteError converts Vault write errors, specifically handling CAS conflicts.
func (s *Store) mapWriteError(err error, path string) error {
	var apiErr *vaultAPIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusPreconditionFailed {
		return &store.ErrCASConflict{Path: path}
	}
	return s.mapError(err)
}

// isVaultNotFound returns true if the error is a Vault 404 response.
func isVaultNotFound(err error) bool {
	var apiErr *vaultAPIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// --- Value encoding/decoding ---

// encodeValue serializes a config.Value into a Vault KV v2 data map.
func encodeValue(v config.Value) (map[string]any, error) {
	valJSON, err := json.Marshal(v.Val)
	if err != nil {
		return nil, fmt.Errorf("marshaling value: %w", err)
	}
	metaJSON, err := json.Marshal(v.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshaling metadata: %w", err)
	}
	return map[string]any{
		"zhi_val":  string(valJSON),
		"zhi_meta": string(metaJSON),
	}, nil
}

// decodeKVResponse parses the KV v2 read response into a config.Value.
// The version is extracted from the Vault response metadata and set on
// the returned Value for automatic optimistic locking on writes.
func decodeKVResponse(resp *vaultResponse) (config.Value, error) {
	if resp == nil || len(resp.Data) == 0 {
		return config.Value{}, nil
	}

	var envelope struct {
		Data     map[string]any `json:"data"`
		Metadata struct {
			Version      json.Number `json:"version"`
			DeletionTime string      `json:"deletion_time"`
			Destroyed    bool        `json:"destroyed"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(resp.Data, &envelope); err != nil {
		return config.Value{}, fmt.Errorf("parsing KV response: %w", err)
	}

	// Soft-deleted or destroyed versions are not found.
	if envelope.Metadata.Destroyed || envelope.Metadata.DeletionTime != "" {
		return config.Value{}, nil
	}

	if envelope.Data == nil {
		return config.Value{}, nil
	}

	var val config.Value

	// Decode the JSON-encoded value.
	if raw, ok := envelope.Data["zhi_val"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &val.Val); err != nil {
			return config.Value{}, fmt.Errorf("decoding value: %w", err)
		}
	}

	// Decode the JSON-encoded metadata.
	if raw, ok := envelope.Data["zhi_meta"].(string); ok && raw != "" {
		var meta map[string]any
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			return config.Value{}, fmt.Errorf("decoding metadata: %w", err)
		}
		val.Metadata = meta
	}

	// Extract version for optimistic locking.
	if v := envelope.Metadata.Version.String(); v != "" && v != "0" {
		val.Version = v
	}

	return val, nil
}

// --- KV metadata parsing ---

// kvMetadata represents the response from GET /v1/{mount}/metadata/{path}.
type kvMetadata struct {
	CurrentVersion int                      `json:"current_version"`
	Versions       map[string]kvVersionMeta `json:"versions"`
}

// kvVersionMeta holds metadata for a single version of a KV v2 secret.
type kvVersionMeta struct {
	CreatedTime  string `json:"created_time"`
	DeletionTime string `json:"deletion_time"`
	Destroyed    bool   `json:"destroyed"`
}

// --- Policy generation ---

// generatePolicy produces a Vault HCL policy document granting the
// specified permissions on paths within a tree.
func generatePolicy(mount, prefix, treeID string, permissions []store.Permission) string {
	var buf strings.Builder
	for _, perm := range permissions {
		vaultPath := perm.Path
		if strings.HasSuffix(vaultPath, "/") {
			vaultPath += "*"
		}

		dataPath := fmt.Sprintf("%s/data/%s/%s/%s", mount, prefix, treeID, vaultPath)
		metaPath := fmt.Sprintf("%s/metadata/%s/%s/%s", mount, prefix, treeID, vaultPath)

		dataCaps := actionsToDataCaps(perm.Actions)
		metaCaps := actionsToMetaCaps(perm.Actions)

		if len(dataCaps) > 0 {
			fmt.Fprintf(&buf, "path %q {\n  capabilities = [%s]\n}\n\n", dataPath, joinCaps(dataCaps))
		}
		if len(metaCaps) > 0 {
			fmt.Fprintf(&buf, "path %q {\n  capabilities = [%s]\n}\n\n", metaPath, joinCaps(metaCaps))
		}
	}
	return buf.String()
}

func actionsToDataCaps(actions []store.Action) []string {
	var caps []string
	for _, a := range actions {
		switch a {
		case store.ActionRead:
			caps = append(caps, "read")
		case store.ActionWrite:
			caps = append(caps, "create", "update")
		case store.ActionDelete:
			caps = append(caps, "delete")
		}
	}
	return caps
}

func actionsToMetaCaps(actions []store.Action) []string {
	var caps []string
	for _, a := range actions {
		switch a {
		case store.ActionRead:
			caps = append(caps, "read", "list")
		case store.ActionDelete:
			caps = append(caps, "delete")
		}
	}
	return caps
}

func joinCaps(caps []string) string {
	quoted := make([]string, len(caps))
	for i, c := range caps {
		quoted[i] = `"` + c + `"`
	}
	return strings.Join(quoted, ", ")
}

// readPolicyPermissions reads a Vault ACL policy and parses it back into
// Permission structs. This is a best-effort parse of the HCL we generate.
func (s *Store) readPolicyPermissions(ctx context.Context, name, treeID string) []store.Permission {
	resp, err := s.client.request(ctx, http.MethodGet, "sys/policies/acl/"+name, nil)
	if err != nil {
		return nil
	}

	var policyData struct {
		Policy string `json:"policy"`
	}
	if json.Unmarshal(resp.Data, &policyData) != nil {
		return nil
	}

	return parsePolicyPermissions(policyData.Policy, s.mount, s.prefix, treeID)
}

// parsePolicyPermissions extracts Permission structs from our generated HCL.
// It matches data path rules and maps Vault capabilities back to Actions.
func parsePolicyPermissions(policy, mount, prefix, treeID string) []store.Permission {
	dataPrefix := fmt.Sprintf("%s/data/%s/%s/", mount, prefix, treeID)

	permMap := make(map[string]map[store.Action]bool)

	// Simple line-by-line parser for our generated policy format.
	lines := strings.Split(policy, "\n")
	var currentPath string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "path ") {
			// Extract the quoted path.
			start := strings.Index(line, `"`)
			end := strings.LastIndex(line, `"`)
			if start >= 0 && end > start {
				vaultPath := line[start+1 : end]
				if p, ok := strings.CutPrefix(vaultPath, dataPrefix); ok {
					p = strings.TrimSuffix(p, "*") // keep trailing /
					currentPath = p
				} else {
					currentPath = ""
				}
			}
		} else if strings.Contains(line, "capabilities") && currentPath != "" {
			actions := capsToActions(line)
			if permMap[currentPath] == nil {
				permMap[currentPath] = make(map[store.Action]bool)
			}
			for _, a := range actions {
				permMap[currentPath][a] = true
			}
			currentPath = ""
		}
	}

	var perms []store.Permission
	for path, actionSet := range permMap {
		var actions []store.Action
		for a := range actionSet {
			actions = append(actions, a)
		}
		slices.Sort(actions)
		perms = append(perms, store.Permission{Path: path, Actions: actions})
	}
	sort.Slice(perms, func(i, j int) bool { return perms[i].Path < perms[j].Path })
	return perms
}

// capsToActions maps a Vault capabilities line back to store.Action values.
func capsToActions(line string) []store.Action {
	var actions []store.Action
	if strings.Contains(line, `"read"`) {
		actions = append(actions, store.ActionRead)
	}
	if strings.Contains(line, `"create"`) || strings.Contains(line, `"update"`) {
		actions = append(actions, store.ActionWrite)
	}
	if strings.Contains(line, `"delete"`) {
		actions = append(actions, store.ActionDelete)
	}
	return actions
}

// mergePermissions combines existing and new permissions, merging actions
// for duplicate paths.
func mergePermissions(existing, incoming []store.Permission) []store.Permission {
	m := make(map[string]map[store.Action]bool)
	for _, p := range existing {
		if m[p.Path] == nil {
			m[p.Path] = make(map[store.Action]bool)
		}
		for _, a := range p.Actions {
			m[p.Path][a] = true
		}
	}
	for _, p := range incoming {
		if m[p.Path] == nil {
			m[p.Path] = make(map[store.Action]bool)
		}
		for _, a := range p.Actions {
			m[p.Path][a] = true
		}
	}

	var result []store.Permission
	for path, actionSet := range m {
		var actions []store.Action
		for a := range actionSet {
			actions = append(actions, a)
		}
		slices.Sort(actions)
		result = append(result, store.Permission{Path: path, Actions: actions})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

// removePermissionPaths filters out permissions for the specified paths.
func removePermissionPaths(perms []store.Permission, paths []string) []store.Permission {
	remove := make(map[string]bool, len(paths))
	for _, p := range paths {
		remove[p] = true
	}
	var result []store.Permission
	for _, p := range perms {
		if !remove[p.Path] {
			result = append(result, p)
		}
	}
	return result
}
