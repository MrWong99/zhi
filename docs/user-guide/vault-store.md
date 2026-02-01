# Vault Store Provider

The built-in `vault` store provider persists configuration trees in
[HashiCorp Vault](https://www.vaultproject.io/) using the KV v2 secret
engine. Each tree entry is stored as its own Vault secret, enabling
fine-grained Vault policies per configuration value.

## Features

- **Per-value secrets** -- Each configuration key is stored at its own
  Vault path. This lets you write Vault policies that grant or restrict
  access to individual configuration values.
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

Each tree entry is stored as a separate Vault secret at
`<mount>/data/<prefix><id>/<config-path>`. The secret's data map contains
a single key (the config path) whose value is a JSON-encoded object with
`value` and optional `metadata` fields.

For example, with the default settings and a tree named `production`:

```
secret/data/zhi/production/pokedex/trainer.name
  data:
    "pokedex/trainer.name": '{"value":"Ash","metadata":{"description":"Name of the Pokemon trainer"}}'

secret/data/zhi/production/pokedex/starter
  data:
    "pokedex/starter": '{"value":"pikachu","metadata":{"description":"Chosen starter Pokemon"}}'

secret/data/zhi/production/pokedex/region
  data:
    "pokedex/region": '{"value":"kanto","metadata":{"description":"Home region of the trainer"}}'

secret/data/zhi/production/games/pokemon_golden/hours.played
  data:
    "games/pokemon_golden/hours.played": '{"value":120}'
```

When a tree is saved, any entries that existed in a previous save but are
no longer present in the tree are automatically deleted from Vault.

## Versioning

Tree-level versioning is **not supported**. Because each configuration
value lives in its own Vault secret, each value has an independent Vault
version history. Coherent tree-level snapshots cannot be reconstructed
from per-value versions.

`SupportsVersioning()` returns `false`, and `ListVersions`, `LoadVersion`,
and `DeleteVersion` return errors.

Individual values still benefit from Vault KV v2's native versioning at
the secret level — you can inspect per-value history directly via the
Vault CLI or API.

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

Because each configuration value is its own Vault secret, you can write
fine-grained policies. A broad policy that grants access to all values:

```hcl
path "<mount>/data/<prefix>*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "<mount>/metadata/<prefix>*" {
  capabilities = ["read", "list", "delete"]
}
```

A more restrictive policy that grants read-only access to a specific
subtree and full access to another:

```hcl
# Read-only access to pokedex configuration.
path "<mount>/data/<prefix>production/pokedex/*" {
  capabilities = ["read"]
}

path "<mount>/metadata/<prefix>production/pokedex/*" {
  capabilities = ["read", "list"]
}

# Full access to games configuration.
path "<mount>/data/<prefix>production/games/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "<mount>/metadata/<prefix>production/games/*" {
  capabilities = ["read", "list", "delete"]
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

Trees will be stored under `secret/data/myproject/<tree-id>/<config-path>`.

## Performance considerations

Because each tree entry is a separate Vault secret, loading and saving a
tree requires one Vault API call per entry plus recursive LIST calls to
discover entries. For trees with many entries, this is more network-
intensive than a single-secret model. The tradeoff is fine-grained access
control at the Vault policy level.
