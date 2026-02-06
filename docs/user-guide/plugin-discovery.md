# Plugin Discovery

zhi discovers external plugins from the filesystem and makes them available alongside built-in providers. External plugins are separate binaries that communicate with zhi over gRPC.

## Plugin Directories

By default, zhi scans `~/.zhi/plugins/` for external plugins. You can configure additional directories in `zhi.yaml`:

```yaml
plugins:
  directories:
    - ~/.zhi/plugins
    - /usr/local/lib/zhi/plugins
    - ./plugins
```

## Naming Convention

External plugin binaries follow the pattern `zhi-<type>-<name>`:

| Binary Name | Type | Provider Name |
|-------------|------|---------------|
| `zhi-config-pokedex` | config | `pokedex` |
| `zhi-transform-evolve` | transform | `evolve` |
| `zhi-store-vault` | store | `vault` |

### Directory Layouts

**Flat layout** (all binaries in one directory):

```
~/.zhi/plugins/
  zhi-config-pokedex
  zhi-transform-evolve
  zhi-store-vault
```

**Type-based subdirectories:**

```
~/.zhi/plugins/
  config/
    pokedex
  transform/
    evolve
  store/
    vault
```

Both layouts are supported. Flat naming takes precedence when both exist.

## Using External Plugins

Reference external plugins by name in `zhi.yaml`, just like built-in providers:

```yaml
config:
  provider: pokedex

store:
  provider: vault
```

The registry resolves providers in two steps:

1. Check built-in providers first
2. If not found, scan plugin directories for a matching binary

External plugins are launched lazily -- only when first used, not at startup.

## Listing Plugins

```sh
$ zhi list providers
CONFIG PROVIDERS:
  structuredfile    (built-in)
  pokedex           (external: ~/.zhi/plugins/zhi-config-pokedex)

TRANSFORM PROVIDERS:
  evolve            (external: ~/.zhi/plugins/zhi-transform-evolve)

STORE PROVIDERS:
  zhi-store-json    (external: ~/.zhi/plugins/zhi-store-json)
```

## Installing Plugins

Place plugin binaries in `~/.zhi/plugins/` (or any configured directory) and ensure they are executable:

```sh
cp zhi-config-myplugin ~/.zhi/plugins/
chmod +x ~/.zhi/plugins/zhi-config-myplugin
```

## Sharing and Installing from OCI Registries

Plugins can be installed directly from OCI registries:

```sh
# Install a plugin from an OCI reference
zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0

# List installed shared plugins (includes signing status)
zhi plugin list

# Show detailed plugin info including signer identity and binary digest
zhi plugin info ansible-config

# Verify a plugin's signature without installing
zhi plugin verify oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0

# Uninstall a shared plugin
zhi plugin uninstall ansible-config
```

## Security

- zhi logs the full path and SHA-256 hash of each launched plugin for audit purposes
- **Binary integrity verification**: At launch time, the binary's SHA-256 digest is compared against the digest recorded during installation. A mismatch indicates post-install tampering and is logged as an error
- **Signature verification**: During `zhi plugin install`, artifact signatures are checked against the security policy. Use `--skip-verify` to bypass (not recommended)
- **Security policy**: Configure `~/.zhi/policy.yaml` to require signatures (`requireSignatures: true`), restrict allowed registries, and block specific plugins
- **Trust store**: Trusted signing keys are managed in `~/.zhi/keys/`
- World-writable plugin binaries produce a warning
- Plugins run as separate processes communicating over stdio (local process boundary)
- The [hashicorp/go-plugin](https://github.com/hashicorp/go-plugin) handshake provides basic identity verification

## Troubleshooting

- **Plugin not found**: Verify the binary is in a configured plugin directory and is executable
- **Handshake failure**: Ensure the plugin uses the correct handshake (`ZHI_PLUGIN=zhiplugin-v1`, protocol version 1)
- **Plugin crash**: zhi detects process exits and reports errors. Check `--verbose` output for details
- **Timeout**: Plugin connections time out after 10 seconds by default
