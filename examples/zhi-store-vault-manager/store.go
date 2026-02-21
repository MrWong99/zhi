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

// Login authenticates to Vault via the admin client. The child plugin
// does not receive the admin credentials -- it gets scoped tokens per operation.
func (s *vaultManagerStore) Login(ctx context.Context, method string, credentials map[string]string) (*store.Credential, error) {
	auth, err := s.admin.login(ctx, method, credentials)
	if err != nil {
		return nil, &store.ErrAuthRequired{Reason: fmt.Sprintf("admin login failed: %v", err)}
	}

	s.log.Info("admin authenticated to Vault", "method", method)

	return &store.Credential{
		Token:    auth.ClientToken,
		Metadata: map[string]string{"admin": "true"},
	}, nil
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
