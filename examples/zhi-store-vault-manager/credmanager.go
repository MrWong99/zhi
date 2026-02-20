package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/labels"
)

// appCredentials holds generated credentials for one app.
type appCredentials struct {
	AuthMethod      string
	RoleID          string // approle only
	WrappedSecretID string // approle only
	Token           string // token auth only
}

// credManager orchestrates Vault policy, AppRole, and credential management.
type credManager struct {
	admin     *adminClient
	mount     string
	prefix    string
	workspace string
	log       hclog.Logger
}

func newCredManager(admin *adminClient, mount, prefix, workspace string, log hclog.Logger) *credManager {
	if log == nil {
		log = hclog.Default()
	}
	return &credManager{
		admin:     admin,
		mount:     mount,
		prefix:    prefix,
		workspace: workspace,
		log:       log,
	}
}

// reconcilePolicies creates/updates/deletes Vault policies based on tree labels.
func (cm *credManager) reconcilePolicies(ctx context.Context, values map[string]config.Value, apps []appConfig) error {
	policyPrefix := fmt.Sprintf("zhi-%s-", cm.workspace)

	// Build desired policies.
	desired := make(map[string]string) // policy name -> HCL
	for _, app := range apps {
		name := policyName(cm.workspace, app.Name)
		hcl := buildPolicyHCL(app.Name, values, cm.mount, cm.prefix, cm.treeID(values))
		desired[name] = hcl
	}

	// List existing managed policies.
	existing, err := cm.admin.listPolicies(ctx, policyPrefix)
	if err != nil {
		return fmt.Errorf("listing existing policies: %w", err)
	}

	// Create or update desired policies.
	var errs []error
	for name, hcl := range desired {
		if err := cm.admin.putPolicy(ctx, name, hcl); err != nil {
			if cm.log != nil {
				cm.log.Error("failed to write policy", "policy", name, "error", err)
			}
			errs = append(errs, fmt.Errorf("policy %s: %w", name, err))
		} else if cm.log != nil {
			cm.log.Info("policy written", "policy", name)
		}
	}

	// Delete stale policies.
	for _, name := range existing {
		if _, ok := desired[name]; !ok {
			if err := cm.admin.deletePolicy(ctx, name); err != nil {
				if cm.log != nil {
					cm.log.Warn("failed to delete stale policy", "policy", name, "error", err)
				}
			} else if cm.log != nil {
				cm.log.Info("stale policy deleted", "policy", name)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("policy reconciliation had %d errors", len(errs))
	}
	return nil
}

// ensureAppRoles creates or updates AppRole roles for approle-type apps.
func (cm *credManager) ensureAppRoles(ctx context.Context, apps []appConfig) error {
	var errs []error
	for _, app := range apps {
		if app.Auth != "approle" {
			continue
		}
		roleName := policyName(cm.workspace, app.Name)
		polName := policyName(cm.workspace, app.Name)

		existing, err := cm.admin.readAppRole(ctx, roleName)
		if err != nil {
			// Role doesn't exist, create it.
			if cm.log != nil {
				cm.log.Info("creating AppRole", "role", roleName)
			}
			params := map[string]any{
				"token_policies": []string{polName},
			}
			if err := cm.admin.writeAppRole(ctx, roleName, params); err != nil {
				if cm.log != nil {
					cm.log.Error("failed to create AppRole", "role", roleName, "error", err)
				}
				errs = append(errs, fmt.Errorf("approle %s: %w", roleName, err))
			}
			continue
		}

		// Role exists -- ensure our policy is in the list.
		policies := extractStringSlice(existing, "token_policies")
		found := false
		for _, p := range policies {
			if p == polName {
				found = true
				break
			}
		}
		if !found {
			policies = append(policies, polName)
			params := map[string]any{
				"token_policies": policies,
			}
			if err := cm.admin.writeAppRole(ctx, roleName, params); err != nil {
				if cm.log != nil {
					cm.log.Error("failed to update AppRole policies", "role", roleName, "error", err)
				}
				errs = append(errs, fmt.Errorf("approle %s update: %w", roleName, err))
			} else if cm.log != nil {
				cm.log.Info("AppRole policy added", "role", roleName, "policy", polName)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("AppRole management had %d errors", len(errs))
	}
	return nil
}

// generateCredentials creates fresh credentials for each app.
func (cm *credManager) generateCredentials(ctx context.Context, apps []appConfig) (map[string]*appCredentials, error) {
	creds := make(map[string]*appCredentials)
	var errs []error

	for _, app := range apps {
		roleName := policyName(cm.workspace, app.Name)

		switch app.Auth {
		case "approle":
			roleID, err := cm.admin.readRoleID(ctx, roleName)
			if err != nil {
				if cm.log != nil {
					cm.log.Error("failed to read role_id", "app", app.Name, "error", err)
				}
				errs = append(errs, fmt.Errorf("app %s role_id: %w", app.Name, err))
				continue
			}

			wrapTTL := app.WrapTTL
			if !app.Wrapped {
				wrapTTL = ""
			}

			wrappedSecret, err := cm.admin.generateWrappedSecretID(ctx, roleName, wrapTTL)
			if err != nil {
				if cm.log != nil {
					cm.log.Error("failed to generate wrapped secret_id", "app", app.Name, "error", err)
				}
				errs = append(errs, fmt.Errorf("app %s secret_id: %w", app.Name, err))
				continue
			}

			creds[app.Name] = &appCredentials{
				AuthMethod:      "approle",
				RoleID:          roleID,
				WrappedSecretID: wrappedSecret,
			}

		case "token":
			polName := policyName(cm.workspace, app.Name)
			policies := append([]string{polName}, app.TokenPolicies...)

			token, err := cm.admin.createToken(ctx, policies, app.TokenTTL)
			if err != nil {
				if cm.log != nil {
					cm.log.Error("failed to create token", "app", app.Name, "error", err)
				}
				errs = append(errs, fmt.Errorf("app %s token: %w", app.Name, err))
				continue
			}

			creds[app.Name] = &appCredentials{
				AuthMethod: "token",
				Token:      token,
			}
		}

		if cm.log != nil {
			cm.log.Info("credentials generated", "app", app.Name, "auth", app.Auth)
		}
	}

	if len(errs) > 0 {
		return creds, fmt.Errorf("credential generation had %d errors (partial results available)", len(errs))
	}
	return creds, nil
}

// injectCredentials adds ephemeral credential values to the values map.
func injectCredentials(values map[string]config.Value, creds map[string]*appCredentials) map[string]config.Value {
	result := make(map[string]config.Value, len(values)+len(creds)*3)
	for k, v := range values {
		result[k] = v
	}

	for appName, c := range creds {
		prefix := "vault/credentials/" + appName + "/"

		meta := func(desc string) map[string]any {
			return labels.NewBuilder().
				Hidden().
				Writeonly().
				Ephemeral().
				Readonly().
				Description(desc).
				Build()
		}

		result[prefix+"auth-method"] = config.Value{
			Val:      c.AuthMethod,
			Metadata: meta(fmt.Sprintf("Auth method for %s", appName)),
		}

		switch c.AuthMethod {
		case "approle":
			result[prefix+"role-id"] = config.Value{
				Val:      c.RoleID,
				Metadata: meta(fmt.Sprintf("Vault AppRole role_id for %s", appName)),
			}
			result[prefix+"wrapped-secret-id"] = config.Value{
				Val:      c.WrappedSecretID,
				Metadata: meta(fmt.Sprintf("Vault response-wrapped secret_id for %s (single-use)", appName)),
			}
		case "token":
			result[prefix+"token"] = config.Value{
				Val:      c.Token,
				Metadata: meta(fmt.Sprintf("Vault token for %s", appName)),
			}
		}
	}

	return result
}

// treeID derives the tree ID from workspace config.
func (cm *credManager) treeID(_ map[string]config.Value) string {
	return cm.workspace
}

func extractStringSlice(m map[string]any, key string) []string {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func startsWith(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func lastSegment(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
