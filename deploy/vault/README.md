# Vault Deployment Workspace

A zhi workspace for deploying and bootstrapping a HashiCorp Vault instance with sensible defaults, zero-touch initialization, and automatic cleanup of sensitive data.

## Overview

This workspace provides a complete Vault deployment solution that:

- **Deploys Vault** via Docker Compose, Kubernetes, or host binary
- **Initializes and unseals** the Vault automatically
- **Creates an admin user** with a comprehensive policy
- **Enables secret engines** (KV v2, Transit, PKI, TOTP, etc.)
- **Revokes the root token** after bootstrap for security
- **Cleans up sensitive files** after the apply script runs

## Prerequisites

### Required Tools (all modes)

- `curl` -- for Vault API interaction
- `jq` -- for JSON processing

### Mode-Specific Requirements

| Mode | Requirements |
|------|--------------|
| `docker` | Docker with the Compose plugin (`docker compose`) |
| `host` | Vault binary installed and in PATH |
| `kubernetes` | `kubectl` with cluster access |

## Quick Start

1. Navigate to the workspace directory:

   ```bash
   cd deploy/vault
   ```

2. Start the zhi UI:

   ```bash
   zhi
   ```

3. Configure your deployment:
   - Set the deployment mode (`docker`, `host`, or `kubernetes`)
   - Set a strong admin password (required)
   - Adjust other settings as needed

4. Export and apply:

   ```bash
   zhi export
   zhi apply
   ```

   Or use the UI to export and apply directly.

5. Save the unseal keys and root token printed to the console -- they are shown only once!

## Configuration Sections

### Server (`vault-server/`)

Core Vault server settings:

| Setting | Default | Description |
|---------|---------|-------------|
| `api-addr` | `http://127.0.0.1:8200` | Vault API address |
| `cluster-addr` | `http://127.0.0.1:8201` | Cluster communication address (HA) |
| `log-level` | `info` | Log verbosity (trace/debug/info/warn/error) |
| `storage-backend` | `file` | Storage backend (file/raft/consul) |
| `storage-path` | `/opt/vault/data` | Filesystem path for storage |
| `tls/enabled` | `false` | Enable TLS for the API listener |
| `ui` | `true` | Enable the Vault web UI |

### Deployment (`vault-deploy/`)

Deployment mode and environment settings:

| Setting | Default | Description |
|---------|---------|-------------|
| `mode` | `docker` | Deployment mode (host/docker/kubernetes) |
| `vault-version` | `1.19.0` | Vault version (image tag) |
| `unseal-shares` | `5` | Number of unseal key shares |
| `unseal-threshold` | `3` | Keys required to unseal |
| `docker/port` | `8200` | Host port for Docker |
| `docker/volume-name` | `vault-data` | Docker volume name |
| `k8s/namespace` | `vault` | Kubernetes namespace |
| `k8s/replicas` | `1` | Number of Vault replicas |
| `k8s/storage-class` | `standard` | StorageClass for PVC |
| `k8s/storage-size` | `10Gi` | PVC size |
| `k8s/service-type` | `ClusterIP` | Service type |
| `k8s/ingress-enabled` | `false` | Create an Ingress |

### Authentication (`vault-auth/`)

Auth methods and initial admin user:

| Setting | Default | Description |
|---------|---------|-------------|
| `methods` | `userpass` | Auth methods to enable (comma-separated) |
| `admin-username` | `admin` | Initial admin username |
| `admin-password` | *(required)* | Admin password (masked in UI) |

Supported auth methods: `userpass`, `approle`, `ldap`, `oidc`, `jwt`, `cert`, `github`, `token`.

### Secret Engines (`vault-secrets/`)

This component is optional and can be disabled in the UI.

| Setting | Default | Description |
|---------|---------|-------------|
| `engines` | `kv-v2` | Engines to enable (comma-separated) |
| `kv-v2-path` | `secret` | Mount path for KV v2 |
| `transit-path` | `transit` | Mount path for Transit |
| `pki-path` | `pki` | Mount path for PKI |
| `totp-path` | `totp` | Mount path for TOTP |

Supported engines: `kv-v2`, `kv-v1`, `transit`, `pki`, `totp`, `ssh`, `aws`, `gcp`, `azure`, `database`, `rabbitmq`, `consul`.

## Deployment Modes

### Docker (default)

Uses Docker Compose to run Vault in a container. Best for development and testing.

```bash
# Generated files:
# - docker-compose.yml
# - vault-config.hcl
# - apply.sh
```

### Host

Runs Vault as a local process using the Vault binary. The binary must be installed and in your PATH.

```bash
# Generated files:
# - vault-config.hcl
# - apply.sh
```

### Kubernetes

Deploys Vault to a Kubernetes cluster using kubectl. Supports StatefulSet, PVC, and optional Ingress.

```bash
# Generated files:
# - k8s-namespace.yml
# - k8s-configmap.yml
# - k8s-statefulset.yml
# - k8s-service.yml
# - k8s-ingress.yml (if ingress enabled)
# - apply.sh
```

## Security

### No Persistent Store

This workspace intentionally has **no store provider**. It is designed for ephemeral use:

1. Configure values via the UI
2. Export templates
3. Run the apply script
4. Discard the workspace

### Secret Cleanup

The apply script automatically deletes itself and other sensitive files after execution. Unseal keys and the root token are only printed to stdout -- never written to disk.

### Root Token Revocation

After bootstrap completes, the root token is revoked. All subsequent administration is done via the admin user you configured.

### Admin Policy

The admin user receives a policy that allows:

- Managing auth methods and users
- Managing policies
- Managing secret engines
- Reading system health and configuration
- Managing identities
- Full access to all secret engines
- Token management

The admin policy does **not** include access to `sys/raw` or `sys/seal` for safety.

## Post-Deployment

### Login as Admin

After deployment, log in using the admin credentials:

```bash
curl -s -X POST http://127.0.0.1:8200/v1/auth/userpass/login/admin \
  -d '{"password": "your-password-here"}' | jq
```

The response contains a `client_token` you can use for subsequent requests:

```bash
export VAULT_TOKEN="<client_token from response>"
curl -s -H "X-Vault-Token: $VAULT_TOKEN" http://127.0.0.1:8200/v1/sys/health | jq
```

### Store Unseal Keys Securely

The unseal keys printed during initialization are required to unseal Vault after a restart. Store them securely:

- Use a password manager
- Split keys among trusted parties
- Consider using Vault's auto-unseal with a cloud KMS for production

### Enable Additional Auth Methods

Add more auth methods as needed:

```bash
# Enable AppRole for service authentication
curl -s -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
  http://127.0.0.1:8200/v1/sys/auth/approle \
  -d '{"type": "approle"}'
```

## Testing with CLI

You can test the workspace using the `zhi export` and `zhi apply` commands directly:

```bash
cd deploy/vault

# Export all templates (dry-run to see output without writing files)
zhi export --dry-run

# Export all templates to disk
zhi export

# Apply (will run pre-export automatically)
zhi apply

# Or run with explicit export step
zhi export && bash ./apply.sh
```

## Troubleshooting

### Vault Not Starting

Check the logs:

```bash
# Docker
docker compose logs vault

# Kubernetes
kubectl -n vault logs statefulset/vault
```

### Initialization Fails

Ensure Vault is running and accessible:

```bash
curl -s http://127.0.0.1:8200/v1/sys/health
```

A 501 response means Vault is running but not initialized (expected before apply).

### Permission Denied

Ensure the apply script is executable:

```bash
chmod +x apply.sh
```

The template includes `fileMode 0700` but this may not work on all filesystems.

## Files Generated

| File | Description |
|------|-------------|
| `apply.sh` | Main bootstrap script (deleted after run) |
| `vault-config.hcl` | Vault server configuration |
| `docker-compose.yml` | Docker Compose manifest |
| `k8s-namespace.yml` | Kubernetes Namespace |
| `k8s-configmap.yml` | Kubernetes ConfigMap |
| `k8s-statefulset.yml` | Kubernetes StatefulSet |
| `k8s-service.yml` | Kubernetes Service |
| `k8s-ingress.yml` | Kubernetes Ingress (if enabled) |

## License

This workspace is part of the zhi project. See the project LICENSE for details.