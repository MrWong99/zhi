package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/MrWong99/zhi/pkg/providers/store/vault/httpclient"
)

// vaultProbe bundles the resolved Vault connection settings used by every
// vault doctor check.
type vaultProbe struct {
	addr      string
	token     string
	namespace string
	mount     string
	prefix    string
	client    *httpclient.Client
}

// vaultSetup extracts the connection settings from the environment and
// builds a shared httpclient. When the configured store isn't vault or no
// workspace is present, it returns a populated skipped Result so the
// caller can short-circuit.
//
// NOTE: TLS config (VAULT_CACERT, VAULT_CLIENT_CERT, VAULT_CLIENT_KEY,
// VAULT_SKIP_VERIFY) is intentionally not wired up in this v1 — doctor
// probes assume plaintext or the system trust store. A full TLS probe
// can be added later by re-using tlsutil.ClientConfig.
func vaultSetup(env *Environment) (*vaultProbe, *Result) {
	if env == nil || env.Workspace == nil || env.Workspace.Store.Provider != "vault" {
		skipped := &Result{
			Status:  StatusSkipped,
			Message: "store provider is not vault",
		}
		return nil, skipped
	}

	opts := env.Workspace.Store.Options

	addr, _ := stringOpt(opts, "addr")
	if addr == "" {
		if v := os.Getenv("VAULT_ADDR"); v != "" {
			addr = v
		} else {
			addr = "http://127.0.0.1:8200"
		}
	}

	namespace, _ := stringOpt(opts, "namespace")
	if namespace == "" {
		namespace = os.Getenv("VAULT_NAMESPACE")
	}

	mount, _ := stringOpt(opts, "mount")
	if mount == "" {
		mount = "secret"
	}
	prefix, _ := stringOpt(opts, "prefix")
	if prefix == "" {
		prefix = "zhi"
	}

	// auth.credentials.token is the nested map path the vault provider
	// uses in zhi.yaml. Any missing level falls back to env.
	token := nestedString(opts, "auth", "credentials", "token")
	if token == "" {
		token = os.Getenv("VAULT_TOKEN")
	}

	timeout := env.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	httpClient := &http.Client{Timeout: timeout}
	client := httpclient.New(addr, token, namespace, httpClient)

	return &vaultProbe{
		addr:      addr,
		token:     token,
		namespace: namespace,
		mount:     mount,
		prefix:    prefix,
		client:    client,
	}, nil
}

// stringOpt returns opts[key] when present and a string.
func stringOpt(opts map[string]any, key string) (string, bool) {
	if opts == nil {
		return "", false
	}
	v, ok := opts[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// nestedString walks a nested map[string]any by keys, returning the final
// string value if every level is present. Tolerates missing levels and
// non-map intermediate types.
func nestedString(opts map[string]any, keys ...string) string {
	cur := any(opts)
	for i, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		v, ok := m[k]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			s, _ := v.(string)
			return s
		}
		cur = v
	}
	return ""
}

// sealStatus is the subset of sys/seal-status we care about. Vault
// returns these at the top level (not wrapped in "data") so we must
// bypass httpclient.Client for this endpoint.
type sealStatus struct {
	Sealed      bool `json:"sealed"`
	Initialized bool `json:"initialized"`
}

// fetchSealStatus makes a plain HTTP GET to /v1/sys/seal-status. We do a
// raw request because Vault returns the fields at the top level of the
// body — httpclient.Client would try to unmarshal them into the standard
// Response envelope (Data json.RawMessage).
func fetchSealStatus(ctx context.Context, p *vaultProbe) (*sealStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.addr+"/v1/sys/seal-status", nil)
	if err != nil {
		return nil, err
	}
	if p.namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.namespace)
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusServiceUnavailable {
		// 503 is expected when Vault is sealed, so we still parse the body.
		return nil, fmt.Errorf("sys/seal-status returned %d: %s", resp.StatusCode, string(body))
	}
	var ss sealStatus
	if err := json.Unmarshal(body, &ss); err != nil {
		return nil, fmt.Errorf("parsing sys/seal-status: %w", err)
	}
	return &ss, nil
}

// tokenLookup mirrors the fields of auth/token/lookup-self we need.
type tokenLookup struct {
	TTL        int    `json:"ttl"`
	ExpireTime string `json:"expire_time"`
	Renewable  bool   `json:"renewable"`
}

func fetchTokenLookup(ctx context.Context, p *vaultProbe) (*tokenLookup, error) {
	resp, err := p.client.Request(ctx, http.MethodGet, "auth/token/lookup-self", nil)
	if err != nil {
		return nil, err
	}
	var tl tokenLookup
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &tl); err != nil {
			return nil, fmt.Errorf("parsing token lookup: %w", err)
		}
	}
	return &tl, nil
}

type vaultReachableCheck struct{}

func (vaultReachableCheck) ID() string         { return "store.vault.reachable" }
func (vaultReachableCheck) Category() Category { return CategoryStore }
func (vaultReachableCheck) Description() string {
	return "Vault sys/seal-status endpoint responds over HTTP."
}

func (c vaultReachableCheck) Run(ctx context.Context, env *Environment) []Result {
	res := Result{CheckID: c.ID(), Name: c.ID()}
	probe, skipped := vaultSetup(env)
	if skipped != nil {
		skipped.CheckID = c.ID()
		skipped.Name = c.ID()
		return []Result{*skipped}
	}
	if _, err := fetchSealStatus(ctx, probe); err != nil {
		res.Status = StatusError
		res.Message = "vault unreachable"
		res.Detail = err.Error()
		res.Hint = "check addr and network connectivity"
		return []Result{res}
	}
	res.Status = StatusOK
	res.Message = "vault reachable at " + probe.addr
	return []Result{res}
}

type vaultUnsealedCheck struct{}

func (vaultUnsealedCheck) ID() string         { return "store.vault.unsealed" }
func (vaultUnsealedCheck) Category() Category { return CategoryStore }
func (vaultUnsealedCheck) Description() string {
	return "Vault reports initialized=true and sealed=false."
}

func (c vaultUnsealedCheck) Run(ctx context.Context, env *Environment) []Result {
	res := Result{CheckID: c.ID(), Name: c.ID()}
	probe, skipped := vaultSetup(env)
	if skipped != nil {
		skipped.CheckID = c.ID()
		skipped.Name = c.ID()
		return []Result{*skipped}
	}
	ss, err := fetchSealStatus(ctx, probe)
	if err != nil {
		res.Status = StatusSkipped
		res.Message = "skipped, see store.vault.reachable"
		return []Result{res}
	}
	if !ss.Initialized {
		res.Status = StatusError
		res.Message = "vault is not initialized"
		res.Hint = "vault operator init"
		return []Result{res}
	}
	if ss.Sealed {
		res.Status = StatusError
		res.Message = "vault is sealed"
		res.Hint = "vault operator unseal"
		return []Result{res}
	}
	res.Status = StatusOK
	res.Message = "vault initialized and unsealed"
	return []Result{res}
}

type vaultTokenValidCheck struct{}

func (vaultTokenValidCheck) ID() string         { return "store.vault.token_valid" }
func (vaultTokenValidCheck) Category() Category { return CategoryStore }
func (vaultTokenValidCheck) Description() string {
	return "Vault token authenticates via auth/token/lookup-self."
}

func (c vaultTokenValidCheck) Run(ctx context.Context, env *Environment) []Result {
	res := Result{CheckID: c.ID(), Name: c.ID()}
	probe, skipped := vaultSetup(env)
	if skipped != nil {
		skipped.CheckID = c.ID()
		skipped.Name = c.ID()
		return []Result{*skipped}
	}
	if probe.token == "" {
		res.Status = StatusError
		res.Message = "no VAULT_TOKEN or auth.credentials.token set"
		res.Hint = "provide a token via env or workspace config"
		return []Result{res}
	}
	if _, err := fetchTokenLookup(ctx, probe); err != nil {
		res.Status = StatusError
		res.Message = "token lookup failed"
		res.Detail = err.Error()
		res.Hint = "generate or refresh your Vault token"
		return []Result{res}
	}
	res.Status = StatusOK
	res.Message = "token authenticates successfully"
	return []Result{res}
}

type vaultTokenTTLCheck struct{}

func (vaultTokenTTLCheck) ID() string         { return "store.vault.token_ttl" }
func (vaultTokenTTLCheck) Category() Category { return CategoryStore }
func (vaultTokenTTLCheck) Description() string {
	return "Vault token has sufficient remaining TTL."
}

func (c vaultTokenTTLCheck) Run(ctx context.Context, env *Environment) []Result {
	res := Result{CheckID: c.ID(), Name: c.ID()}
	probe, skipped := vaultSetup(env)
	if skipped != nil {
		skipped.CheckID = c.ID()
		skipped.Name = c.ID()
		return []Result{*skipped}
	}
	if probe.token == "" {
		res.Status = StatusSkipped
		res.Message = "no token configured, see store.vault.token_valid"
		return []Result{res}
	}
	tl, err := fetchTokenLookup(ctx, probe)
	if err != nil {
		res.Status = StatusSkipped
		res.Message = "skipped, see store.vault.token_valid"
		return []Result{res}
	}

	// TTL 0 is idiomatic for root / no-expiry tokens.
	if tl.TTL == 0 {
		res.Status = StatusOK
		res.Message = "token has no expiry (root-like)"
		return []Result{res}
	}

	ttl := time.Duration(tl.TTL) * time.Second
	ttlStr := formatDuration(ttl)
	if tl.ExpireTime != "" {
		if exp, err := time.Parse(time.RFC3339, tl.ExpireTime); err == nil {
			ttlStr = fmt.Sprintf("%s (expires %s)", ttlStr, exp.Format(time.RFC3339))
		}
	}

	if tl.TTL <= 3600 {
		res.Status = StatusWarning
		res.Message = fmt.Sprintf("TTL %s, consider renewing", ttlStr)
		res.Hint = "vault token renew"
		return []Result{res}
	}
	res.Status = StatusOK
	res.Message = "TTL: " + ttlStr
	return []Result{res}
}

type vaultMountAccessibleCheck struct{}

func (vaultMountAccessibleCheck) ID() string         { return "store.vault.mount_accessible" }
func (vaultMountAccessibleCheck) Category() Category { return CategoryStore }
func (vaultMountAccessibleCheck) Description() string {
	return "The configured Vault mount/prefix is readable by the current token."
}

func (c vaultMountAccessibleCheck) Run(ctx context.Context, env *Environment) []Result {
	res := Result{CheckID: c.ID(), Name: c.ID()}
	probe, skipped := vaultSetup(env)
	if skipped != nil {
		skipped.CheckID = c.ID()
		skipped.Name = c.ID()
		return []Result{*skipped}
	}

	listPath := probe.mount + "/metadata/" + probe.prefix + "/"
	resp, err := probe.client.List(ctx, listPath)
	if err != nil {
		var apiErr *httpclient.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case http.StatusNotFound:
				// Empty KV v2 mount — still accessible, just no data yet.
				res.Status = StatusOK
				res.Message = "mount accessible (no trees yet)"
				return []Result{res}
			case http.StatusForbidden:
				res.Status = StatusError
				res.Message = "token lacks read permission on mount"
				res.Detail = apiErr.Error()
				res.Hint = "grant `list` on this path"
				return []Result{res}
			}
		}
		res.Status = StatusError
		res.Message = "LIST on mount failed"
		res.Detail = err.Error()
		return []Result{res}
	}

	keys, _ := httpclient.ParseListKeys(resp)
	res.Status = StatusOK
	res.Message = fmt.Sprintf("%d tree(s) under %s", len(keys), listPath)
	return []Result{res}
}

// formatDuration prints a compact, human-friendly duration without the
// default Go "0s" noise (e.g. "1h0m0s" → "1h").
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	out := ""
	if h > 0 {
		out += fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		out += fmt.Sprintf("%dm", m)
	}
	if s > 0 || out == "" {
		out += fmt.Sprintf("%ds", s)
	}
	return out
}
