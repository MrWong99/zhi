package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// vaultManagerStore wraps a child vault store plugin with credential management.
type vaultManagerStore struct {
	store.DelegatingPlugin

	admin       *adminClient
	credManager *credManager
	apps        []appConfig
	cfg         *managerConfig
	log         hclog.Logger
}

// Login authenticates to Vault via the admin client, then creates a scoped
// child token with limited permissions and authenticates the child plugin.
func (s *vaultManagerStore) Login(ctx context.Context, method string, credentials map[string]string) (*store.Credential, error) {
	auth, err := s.admin.login(ctx, method, credentials)
	if err != nil {
		return nil, &store.ErrAuthRequired{Reason: fmt.Sprintf("admin login failed: %v", err)}
	}

	s.log.Info("admin authenticated to Vault", "method", method)

	// Create a scoped ACL policy for the child plugin.
	policyName := "zhi-child-" + s.cfg.Mount + "-" + s.cfg.Prefix
	policyHCL := buildChildStorePolicy(s.cfg.Mount, s.cfg.Prefix)
	if err := s.admin.putPolicy(ctx, policyName, policyHCL); err != nil {
		return nil, fmt.Errorf("creating child policy: %w", err)
	}

	// Create a scoped token for the child plugin.
	childToken, err := s.admin.createToken(ctx, []string{policyName}, "1h")
	if err != nil {
		return nil, fmt.Errorf("creating child token: %w", err)
	}

	// Authenticate the child plugin with the scoped token.
	if _, err := s.Base.Login(ctx, "token", map[string]string{"token": childToken}); err != nil {
		return nil, fmt.Errorf("authenticating child plugin: %w", err)
	}

	s.log.Info("child plugin authenticated with scoped token")

	return &store.Credential{
		Token:    auth.ClientToken,
		Metadata: map[string]string{"admin": "true"},
	}, nil
}

// buildChildStorePolicy generates a Vault ACL policy HCL granting KV v2 CRUD
// on the child plugin's mount and prefix paths.
func buildChildStorePolicy(mount, prefix string) string {
	return fmt.Sprintf(`path "%s/data/%s/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "%s/metadata/%s/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "%s/destroy/%s/*" {
  capabilities = ["create", "update"]
}
`, mount, prefix, mount, prefix, mount, prefix)
}

// GetValues retrieves values from the child plugin, then injects
// ephemeral credentials if credential paths are requested.
func (s *vaultManagerStore) GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error) {
	var realPaths []string
	var credPathsRequested bool
	for _, p := range paths {
		if isCredentialPath(p) {
			credPathsRequested = true
		} else {
			realPaths = append(realPaths, p)
		}
	}

	values, err := s.Base.GetValues(ctx, id, realPaths)
	if err != nil {
		return nil, err
	}

	if credPathsRequested && s.admin.getToken() != "" {
		if err := s.credManager.reconcilePolicies(ctx, values, s.apps); err != nil {
			s.log.Error("policy reconciliation failed", "error", err)
		}
		if err := s.credManager.ensureAppRoles(ctx, s.apps); err != nil {
			s.log.Error("AppRole management failed", "error", err)
		}

		creds, err := s.credManager.generateCredentials(ctx, s.apps)
		if err != nil {
			s.log.Error("credential generation had errors", "error", err)
		}
		if len(creds) > 0 {
			values = injectCredentials(values, creds)
		}
	}

	return values, nil
}

// PutValues filters out ephemeral values before delegating to the child.
func (s *vaultManagerStore) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
	filtered := make(map[string]config.Value, len(values))
	for path, val := range values {
		if labels.IsEphemeral(val.Metadata) {
			if s.log != nil {
				s.log.Debug("skipping ephemeral value", "path", path)
			}
			continue
		}
		filtered[path] = val
	}

	return s.Base.PutValues(ctx, id, filtered, opts)
}

func isCredentialPath(path string) bool {
	return path == "vault/credentials" || strings.HasPrefix(path, "vault/credentials/")
}
