# Vault Credential Management

The `zhi-store-vault-manager` meta-plugin wraps the standard `zhi-store-vault` store with automatic credential lifecycle management for deployed applications. It generates least-privilege Vault policies, creates AppRole roles, and injects ephemeral credentials into the configuration tree -- all driven by metadata labels on your config values.

## Overview

Without the vault-manager, applications that consume secrets from Vault need manually provisioned AppRoles, hand-written policies, and a separate credential delivery pipeline. The vault-manager automates this:

1. You annotate config values with `vault.app.<name>` metadata labels declaring which apps need access and at what privilege level
2. The meta-plugin reads these labels, generates a Vault ACL policy per app, and creates the corresponding AppRole (or token)
3. Fresh credentials are injected as ephemeral values at `vault/credentials/<app>/` paths -- visible in exports but never persisted to the store

The admin token used by the meta-plugin never leaves the plugin process. Child operations against `zhi-store-vault` receive short-lived scoped tokens.

## Prerequisites

- A running HashiCorp Vault instance with the KV v2 secret engine enabled
- The `zhi-store-vault` plugin binary installed (the meta-plugin launches it as a child process)
- The `zhi-store-vault-manager` plugin binary installed alongside `zhi-store-vault`
- A Vault token with sufficient permissions (see [Bootstrapping](#bootstrapping))

Install both plugins:

```sh
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-vault:latest
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-vault-manager:latest
```

## Workspace Configuration

Use `vault-manager` as your store provider instead of `vault`:

```yaml
store:
  provider: vault-manager
  options:
    addr: "http://127.0.0.1:8200"
    mount: "secret"
    prefix: "zhi"
    workspace: "production"
    # auth:
    #   method: token
    #   credentials:
    #     token: "hvs.xxxxx"
```

### Provider Options

| Option | Env Fallback | Default | Description |
|--------|-------------|---------|-------------|
| `addr` | `VAULT_ADDR` | `http://127.0.0.1:8200` | Vault server address |
| `mount` | `ZHI_VAULT_MOUNT` | `secret` | KV v2 mount path |
| `prefix` | `ZHI_VAULT_PREFIX` | `zhi` | Path prefix for all zhi secrets |
| `namespace` | `VAULT_NAMESPACE` | (empty) | Vault Enterprise namespace |
| `workspace` | | `default` | Workspace name for policy/role naming |
| `apps` | | (none) | List of application credential configs (see below) |

### App Configuration

The `apps` list declares which applications should receive credentials. Each entry specifies the auth method and its parameters:

```yaml
store:
  provider: vault-manager
  options:
    addr: "http://127.0.0.1:8200"
    mount: "secret"
    apps:
      - name: web-api
        auth: approle
        wrapped: true
        wrap_ttl: "120s"
      - name: batch-worker
        auth: token
        token_ttl: "1h"
        token_policies:
          - extra-read-policy
```

| Field | Default | Description |
|-------|---------|-------------|
| `name` | (required) | Application name, used in policy and path naming |
| `auth` | `approle` | Auth method: `approle` or `token` |
| `wrapped` | `true` | (approle only) Use response-wrapping for the secret_id |
| `wrap_ttl` | `120s` | (approle only) Wrapping token TTL |
| `token_ttl` | `1h` | (token only) Token lifetime |
| `token_policies` | (none) | (token only) Additional policies beyond the auto-generated one |

## Metadata Labels

The vault-manager uses `vault.app.<name>` metadata labels on config values to determine access policies. The label value is a comma-separated list of capabilities: `read`, `write`, `delete`.

Set labels on config values in your config provider:

```yaml
# config/database/host.yaml
value: "db.example.com"
metadata:
  vault.app.web-api: "read"
  vault.app.batch-worker: "read,write"
```

The meta-plugin translates these into Vault ACL policies:

| Label Capability | Vault Capabilities |
|-----------------|-------------------|
| `read` | `read` on data path, `read` + `list` on metadata path |
| `write` | `create`, `update` on data path |
| `delete` | `delete` on data path |

Policies follow the naming convention `zhi-<workspace>-<app>` (e.g. `zhi-production-web-api`).

## Ephemeral Credentials

Generated credentials appear at `vault/credentials/<app>/` paths in the configuration tree. These values are marked with the `store.ephemeral` label and are never written to the Vault backend.

For an AppRole app, the injected paths are:

| Path | Description |
|------|-------------|
| `vault/credentials/<app>/auth-method` | Always `approle` |
| `vault/credentials/<app>/role-id` | The AppRole role_id |
| `vault/credentials/<app>/wrapped-secret-id` | Response-wrapped secret_id (single-use) |

For a token app:

| Path | Description |
|------|-------------|
| `vault/credentials/<app>/auth-method` | Always `token` |
| `vault/credentials/<app>/token` | A short-lived Vault token |

All credential values carry these labels: `ui.hidden`, `store.writeonly`, `store.ephemeral`, `ui.readonly`, and a `core.description`.

## Bootstrapping

Before using the vault-manager, create a Vault policy for the admin token. The `zhi vault-credentials bootstrap` command generates the required HCL:

```sh
# Preview the policy
zhi vault-credentials bootstrap --dry-run

# Apply it to Vault
vault policy write zhi-vault-manager - <<'EOF'
path "secret/data/zhi/*" {
  capabilities = ["read", "create", "update", "delete", "list"]
}

path "secret/metadata/zhi/*" {
  capabilities = ["read", "list", "delete"]
}

path "sys/policies/acl/zhi-*" {
  capabilities = ["read", "create", "update", "delete", "list"]
}

path "auth/approle/role/zhi-*" {
  capabilities = ["read", "create", "update", "delete", "list"]
}

path "auth/approle/role/zhi-*/secret-id" {
  capabilities = ["create", "update"]
}

path "auth/token/create-orphan" {
  capabilities = ["create", "update"]
}
EOF
```

Then create a token with this policy and configure it in your workspace (or log in interactively):

```sh
vault token create -policy=zhi-vault-manager -ttl=720h
```

## Refreshing Credentials

Use `zhi vault-credentials refresh` to generate fresh credentials without a full export/apply cycle. This is useful during application restarts or rolling deployments:

```sh
# All apps, JSON output
zhi vault-credentials refresh

# Single app, env-var format (for shell eval)
zhi vault-credentials refresh --app web-api --output env

# Quiet mode (just the count)
zhi vault-credentials refresh --output quiet
```

See the [CLI Reference](cli-reference.md#zhi-vault-credentials) for the full flag list.

## Security Model

- **Privilege separation**: The admin token is held only by the meta-plugin process. The child `zhi-store-vault` receives scoped one-time tokens per operation via `WithIsolatedEnv`.
- **Least privilege**: Each app gets a policy granting access only to paths that carry its `vault.app.<name>` label.
- **No credential persistence**: Ephemeral values (`store.ephemeral`) are filtered out during `PutValues`, so credentials exist only in-memory during a session.
- **Policy reconciliation**: Stale policies (for removed apps) are deleted automatically.
- **AppRole management**: Roles are created additively -- existing policies on a role are preserved when adding the zhi-managed policy.

## Architecture

The vault-manager is a [meta-plugin](../plugin-development/meta-plugin.md) built on the SDK's delegate pattern:

```
zhi engine
  └── zhi-store-vault-manager (meta-plugin)
        ├── adminClient (Vault HTTP API)
        ├── credManager (policies, roles, credentials)
        └── zhi-store-vault (child plugin, launched via launch.LaunchStore)
```

All 28 store interface methods are forwarded to the child via `store.DelegatingPlugin`. The meta-plugin overrides three:

- **Login** -- authenticates the admin client, not the child
- **GetValues** -- reconciles policies and injects credentials when `vault/credentials/` paths are requested
- **PutValues** -- filters out `store.ephemeral` values before delegating

## Related Documentation

- [Workspace Configuration](workspace-configuration.md) -- store provider setup and auth options
- [CLI Reference](cli-reference.md#zhi-vault-credentials) -- `zhi vault-credentials` commands
- [Meta-Plugin SDK](../plugin-development/meta-plugin.md) -- how meta-plugins work
- [Store Plugin Development](../plugin-development/store-plugin.md) -- store interface reference
