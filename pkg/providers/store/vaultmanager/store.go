package vaultmanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

// Store wraps a child vault store plugin with credential management.
type Store struct {
	store.DelegatingPlugin

	Admin       *AdminClient
	CredManager *CredManager
	Apps        []AppConfig
	Cfg         *Config
	Log         hclog.Logger
}

// Login authenticates to Vault via the admin client, then creates a scoped
// child token with limited permissions and authenticates the child plugin.
func (s *Store) Login(ctx context.Context, method string, credentials map[string]string) (*store.Credential, error) {
	auth, err := s.Admin.Login(ctx, method, credentials)
	if err != nil {
		return nil, &store.ErrAuthRequired{Reason: fmt.Sprintf("admin login failed: %v", err)}
	}

	if s.Log != nil {
		s.Log.Info("admin authenticated to Vault", "method", method)
	}

	// Create a scoped ACL policy for the child plugin.
	policyName := "zhi-child-" + s.Cfg.Mount + "-" + s.Cfg.Prefix
	policyHCL := BuildChildStorePolicy(s.Cfg.Mount, s.Cfg.Prefix)
	if err := s.Admin.PutPolicy(ctx, policyName, policyHCL); err != nil {
		return nil, fmt.Errorf("creating child policy: %w", err)
	}

	// Create a scoped token for the child plugin.
	childToken, err := s.Admin.CreateToken(ctx, []string{policyName}, "1h")
	if err != nil {
		return nil, fmt.Errorf("creating child token: %w", err)
	}

	// Authenticate the child plugin with the scoped token.
	if _, err := s.Base.Login(ctx, "token", map[string]string{"token": childToken}); err != nil {
		return nil, fmt.Errorf("authenticating child plugin: %w", err)
	}

	if s.Log != nil {
		s.Log.Info("child plugin authenticated with scoped token")
	}

	return &store.Credential{
		Token:    auth.ClientToken,
		Metadata: map[string]string{"admin": "true"},
	}, nil
}

// GetValues retrieves values from the child plugin, then injects
// ephemeral credentials if credential paths are requested.
func (s *Store) GetValues(ctx context.Context, id string, paths []string) (map[string]config.Value, error) {
	var realPaths []string
	var credPathsRequested bool
	for _, p := range paths {
		if IsCredentialPath(p) {
			credPathsRequested = true
		} else {
			realPaths = append(realPaths, p)
		}
	}

	values, err := s.Base.GetValues(ctx, id, realPaths)
	if err != nil {
		return nil, err
	}

	if credPathsRequested && s.Admin.Token() != "" {
		if err := s.CredManager.ReconcilePolicies(ctx, values, s.Apps); err != nil {
			if s.Log != nil {
				s.Log.Error("policy reconciliation failed", "error", err)
			}
		}
		if err := s.CredManager.EnsureAppRoles(ctx, s.Apps); err != nil {
			if s.Log != nil {
				s.Log.Error("AppRole management failed", "error", err)
			}
		}

		creds, err := s.CredManager.GenerateCredentials(ctx, s.Apps)
		if err != nil {
			if s.Log != nil {
				s.Log.Error("credential generation had errors", "error", err)
			}
		}
		if len(creds) > 0 {
			values = InjectCredentials(values, creds)
		}
	}

	return values, nil
}

// PutValues filters out ephemeral values before delegating to the child.
func (s *Store) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
	filtered := make(map[string]config.Value, len(values))
	for path, val := range values {
		if labels.IsEphemeral(val.Metadata) {
			if s.Log != nil {
				s.Log.Debug("skipping ephemeral value", "path", path)
			}
			continue
		}
		filtered[path] = val
	}

	return s.Base.PutValues(ctx, id, filtered, opts)
}

// IsCredentialPath returns true if the path is a vault credential path.
func IsCredentialPath(path string) bool {
	return path == "vault/credentials" || strings.HasPrefix(path, "vault/credentials/")
}
