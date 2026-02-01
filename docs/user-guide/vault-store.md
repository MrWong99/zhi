# Vault Store Provider

The built-in `vault` store provider persists configuration trees in
[HashiCorp Vault](https://www.vaultproject.io/) using the KV v2 secret
engine. Each configuration tree maps to a single Vault secret, with tree
entries stored as key-value pairs in the secret's data.

## Features

- **Versioning** -- Vault KV v2 natively versions secrets, so every
  `Save` creates a new version. You can list, load, and delete individual
  versions through the store plugin interface.
- **Encryption at rest** -- Vault encrypts all data at rest using its
  internal barrier encryption. The provider always reports
  `EncryptionActive`.
- **Standard Vault authentication** -- Supports `VAULT_ADDR`,
  `VAULT_TOKEN`, and `~/.vault-token` out of the box. Any Vault client
  environment variable works.

## Workspace configuration

```yaml
store:
  provider: vault
  options:
    address: https://vault.example.com:8200
    token: s.mytoken           # optional: overrides VAULT_TOKEN
    mount: secret              # KV v2 mount path (default: "secret")
    prefix: zhi/               # path prefix for trees (default: "zhi/")
```

### Options

| Option    | Default      | Description                                                       |
|-----------|--------------|-------------------------------------------------------------------|
| `address` | `VAULT_ADDR` | Vault server address. Falls back to the standard env variable.    |
| `token`   | `VAULT_TOKEN`| Authentication token. Falls back to env variable, then `~/.vault-token`. |
| `mount`   | `secret`     | KV v2 mount path in Vault.                                       |
| `prefix`  | `zhi/`       | Path prefix under the mount. Tree IDs are appended to this.      |

## Storage model

Each tree ID is stored as a Vault secret at `<mount>/data/<prefix><id>`.
For example, with the default settings, a tree named `production` is stored
at `secret/data/zhi/production`.

The secret's data map contains one entry per tree path. Each entry is a
JSON-encoded object with `value` and optional `metadata` fields:

```
secret/data/zhi/production
  data:
    "db/host":  '{"value":"localhost","metadata":{"desc":"Database host"}}'
    "db/port":  '{"value":5432}'
    "app/name": '{"value":"myapp"}'
```

## Versioning

Vault KV v2 assigns an integer version number to each write. The store
plugin maps these directly:

- `SupportsVersioning()` returns `true`
- `ListVersions()` returns Vault version numbers as strings (e.g. `"3"`,
  `"2"`, `"1"`), newest first
- `LoadVersion()` accepts these version strings
- `DeleteVersion()` soft-deletes a version in Vault

## Encryption

Vault encrypts data at rest using its storage backend encryption.
`EncryptionStatus()` always returns `EncryptionActive`.

`InitEncryption` and `RotateEncryption` return errors because Vault manages
its own encryption lifecycle. Use `vault operator rotate` for master key
rotation.

## Prerequisites

1. A running Vault server with a KV v2 secret engine enabled.
2. A valid Vault token with permissions to read, write, list, and delete
   secrets under the configured mount and prefix.

### Minimum Vault policy

```hcl
path "<mount>/data/<prefix>*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "<mount>/metadata/<prefix>*" {
  capabilities = ["read", "list", "delete"]
}

path "<mount>/delete/<prefix>*" {
  capabilities = ["update"]
}
```

Replace `<mount>` and `<prefix>` with your configured values (e.g.
`secret` and `zhi/`).

## Example

Enable the KV v2 engine and configure zhi:

```sh
# Enable KV v2 at the default "secret" mount (if not already enabled).
vault secrets enable -version=2 -path=secret kv

# Set env variables for zhi.
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=s.mytoken

# Configure zhi.yaml
cat > zhi.yaml <<EOF
version: "1"
config:
  provider: structuredfile
store:
  provider: vault
  options:
    prefix: myproject/
EOF
```

Trees will be stored under `secret/data/myproject/<tree-id>`.
