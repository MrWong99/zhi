# CLI Integration

New CLI commands for plugin and workspace management. All commands integrate with the existing Cobra command structure in `internal/cli/`.

## Command Tree

```
zhi plugin
  ├── install <ref>          Install a plugin from an OCI reference or short name
  ├── uninstall <name>       Remove an installed plugin
  ├── list                   List installed plugins (extends existing command)
  ├── search <query>         Search the marketplace for plugins
  ├── info <name>            Show detailed information about a plugin
  ├── update [name]          Update one or all plugins to latest versions
  ├── publish                Publish a plugin to an OCI registry
  ├── init                   Generate a zhi-plugin.yaml manifest for a new plugin
  └── verify <ref>           Verify the signature of a plugin artifact

zhi workspace
  ├── install <ref>          Install a workspace and its plugin dependencies
  ├── publish                Publish the current workspace as a shareable artifact
  ├── init                   Initialize a new workspace with interactive setup
  └── lock                   Generate/update zhi-plugins.lock from zhi.yaml

zhi registry
  ├── login <host>           Authenticate with an OCI registry
  ├── logout <host>          Remove stored credentials
  └── list                   List configured registries
```

## Command Details

### `zhi plugin install`

```
Usage:
  zhi plugin install <reference> [flags]

Arguments:
  reference    OCI reference, short name, or local path
               Examples:
                 oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
                 ansible-config@1.2.0
                 ansible-config           (installs latest)
                 ./local-plugin.oci

Flags:
  --version string     Version constraint (e.g., ">=1.0", "~1.2")
  --platform string    Override platform detection (e.g., "linux/amd64")
  --skip-verify        Skip signature verification (not recommended)
  --dry-run            Show what would be installed without installing
  --force              Overwrite existing plugin even if same version

Examples:
  # Install from marketplace (short name)
  zhi plugin install ansible-config

  # Install specific version
  zhi plugin install ansible-config@1.2.0

  # Install from explicit OCI reference
  zhi plugin install oci://ghcr.io/myorg/zhi-config-custom:v1.0.0

  # Install from local OCI layout (offline)
  zhi plugin install oci-layout:///path/to/bundle/
```

**Output:**

```
Resolving ansible-config@1.2.0...
  → oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0
Pulling for linux/amd64...
  ✓ Plugin binary (14.2 MB)
Verifying signature...
  ✓ Signed by release@zhi.dev (Sigstore/Fulcio)
  ✓ Verified publisher: zhi-project
Installing to ~/.zhi/plugins/zhi-config-ansible...
  ✓ Installed ansible-config v1.2.0

Plugin is ready to use. Add to your workspace:
  config:
    provider: ansible-config
```

### `zhi plugin search`

```
Usage:
  zhi plugin search <query> [flags]

Flags:
  --type string        Filter by plugin type (config, transform, store, ui)
  --sort string        Sort by: relevance, downloads, rating, updated (default: relevance)
  --verified           Show only verified plugins
  --json               Output as JSON
  --limit int          Maximum results (default: 20)

Examples:
  zhi plugin search ansible
  zhi plugin search "vault store" --type store
  zhi plugin search --type ui --sort rating --verified
```

**Output:**

```
NAME                  TYPE        VERSION   RATING   DOWNLOADS   DESCRIPTION
ansible-config        config      1.2.0     ★4.7     12.4k       Ansible inventory config provider
ansible-transform     transform   0.9.1     ★4.2     3.1k        Ansible playbook transforms
vault-store           store       2.0.0     ★4.9     28.7k       HashiCorp Vault KV store ✓
```

The `✓` indicates a verified publisher.

### `zhi plugin info`

```
Usage:
  zhi plugin info <name> [flags]

Flags:
  --versions           Show all available versions
  --json               Output as JSON

Examples:
  zhi plugin info ansible-config
  zhi plugin info ansible-config --versions
```

**Output:**

```
Name:        ansible-config
Type:        config
Author:      zhi-project (verified ✓)
Version:     1.2.0 (latest)
License:     Apache-2.0
Rating:      ★★★★★ 4.7 (89 ratings)
Downloads:   12,450
Homepage:    https://github.com/zhi-project/zhi-config-ansible
Platforms:   linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
Signed:      Yes (release@zhi.dev via Sigstore)
Installed:   Yes (v1.1.0 → update available)

Description:
  Configuration provider that reads from Ansible inventory files.
  Supports INI and YAML inventory formats with group variables.

Dependencies: none
Runtime:      none (native Go binary)
```

### `zhi plugin update`

```
Usage:
  zhi plugin update [name] [flags]

Arguments:
  name         Plugin to update (omit to check/update all)

Flags:
  --check              Only check for updates, don't install
  --all                Update all plugins with available updates
  --pre-release        Include pre-release versions
  --json               Output as JSON

Examples:
  zhi plugin update --check          # Check all plugins
  zhi plugin update ansible-config   # Update specific plugin
  zhi plugin update --all            # Update everything
```

**Output (--check):**

```
Checking for updates...

PLUGIN              INSTALLED   AVAILABLE   REGISTRY
ansible-config      1.1.0       1.2.0       ghcr.io/zhi-project
vault-store         2.0.0       (up to date)

1 update available. Run 'zhi plugin update --all' to install.
```

### `zhi plugin publish`

```
Usage:
  zhi plugin publish [flags]

Flags:
  --registry string    Target OCI registry (default: from config)
  --sign               Sign the artifact with cosign (default: true if key available)
  --key string         Path to cosign private key (or use COSIGN_KEY env)
  --platform strings   Platforms to include (default: auto-detect from build artifacts)
  --tag string         OCI tag (default: from manifest version, e.g., "v1.2.0")
  --register           Also register with central marketplace

Prerequisites:
  - zhi-plugin.yaml must exist in current directory
  - Built binaries for target platforms in dist/ directory

Examples:
  zhi plugin publish --registry ghcr.io/myorg --sign --register
```

**Plugin manifest file (`zhi-plugin.yaml`):**

```yaml
name: my-config-plugin
type: config
version: 1.0.0
description: My custom configuration provider
author: myorg
license: Apache-2.0
homepage: https://github.com/myorg/zhi-config-myplugin
keywords:
  - custom
  - config

# Optional: runtime dependencies
runtime:
  type: java
  version: ">=17"
  bundled: false

# Build artifacts (paths relative to project root)
binaries:
  linux/amd64: dist/zhi-config-myplugin_linux_amd64
  linux/arm64: dist/zhi-config-myplugin_linux_arm64
  darwin/amd64: dist/zhi-config-myplugin_darwin_amd64
  darwin/arm64: dist/zhi-config-myplugin_darwin_arm64
```

### `zhi workspace install`

```
Usage:
  zhi workspace install <reference> [target-dir] [flags]

Arguments:
  reference    OCI reference or short name for the workspace
  target-dir   Directory to install into (default: ./<workspace-name>)

Flags:
  --skip-plugins       Don't install plugin dependencies
  --skip-tools-check   Don't check for required external tools
  --dry-run            Show what would be installed

Examples:
  zhi workspace install k8s-cluster
  zhi workspace install oci://ghcr.io/org/zhi-workspace-k8s:v1.0 ./my-cluster
```

**Output:**

```
Resolving k8s-cluster@1.0.0...
  → oci://ghcr.io/zhi-project/zhi-workspace-k8s:v1.0.0

Installing plugins:
  ✓ zhi-config-structured v1.0.0 (already installed)
  ✓ zhi-transform-k8s v1.0.0 (14.2 MB)
  ✓ zhi-store-vault v2.0.0 (already installed)

Checking tools:
  ✓ kubectl v1.29.0 (>= 1.28 required)
  ⚠ helm not found (>= 3.12 required)
    Install with: brew install helm

Extracting workspace to ./k8s-cluster/...
  ✓ zhi.yaml
  ✓ templates/kubeconfig.tmpl
  ✓ templates/helm-values.yaml.tmpl
  ✓ apply/setup.sh

Workspace ready! Next steps:
  cd k8s-cluster
  zhi run
```

### `zhi workspace publish`

```
Usage:
  zhi workspace publish [flags]

Flags:
  --registry string    Target OCI registry
  --sign               Sign the artifact
  --tag string         OCI tag (default: from workspace version)
  --register           Register with central marketplace

Prerequisites:
  - zhi.yaml must exist in current directory
  - Optional: zhi-workspace.yaml with extra metadata (description, keywords, etc.)
```

### `zhi registry login`

```
Usage:
  zhi registry login <host> [flags]

Flags:
  --username string    Registry username
  --password string    Registry password (or use --password-stdin)
  --password-stdin     Read password from stdin

Examples:
  zhi registry login ghcr.io --username myuser
  echo $GHCR_TOKEN | zhi registry login ghcr.io --username myuser --password-stdin
```

Credentials are stored in `~/.zhi/config.yaml` (encrypted if store plugin supports it) or delegated to the system credential helper (Docker credential helpers are compatible).

## Integration with Existing Commands

### `zhi list providers` (enhanced)

The existing `list providers` command (`internal/cli/list.go`) is extended to show sharing metadata:

```
CONFIG PROVIDERS:
  NAME                SOURCE              VERSION   UPDATE
  structuredfile      built-in            -         -
  ansible-config      ~/.zhi/plugins/     1.1.0     1.2.0 available

STORE PROVIDERS:
  NAME                SOURCE              VERSION   UPDATE
  vault               built-in            -         -
  json-store          ~/.zhi/plugins/     0.5.0     (up to date)
```

### `zhi run` (workspace setup)

When `zhi run` encounters a provider that is not installed but is declared in `zhi.yaml`'s `sharing.plugins` section, it offers to install it:

```
Plugin "ansible-config" (config) is not installed but is declared in zhi.yaml.
Install from oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0? [Y/n]
```

### `zhi workspace lock`

Generates or updates `zhi-plugins.lock` by resolving all plugin references in `zhi.yaml` to exact digests:

```
Usage:
  zhi workspace lock [flags]

Flags:
  --update             Update all plugins to latest matching versions
  --update-plugin string  Update a specific plugin

Examples:
  zhi workspace lock            # Lock current versions
  zhi workspace lock --update   # Update and re-lock
```

## Auto-Completion

All commands support shell auto-completion for plugin names (from marketplace and installed plugins), OCI references (from configured registries), and version tags.

```bash
# Add to ~/.bashrc or ~/.zshrc
eval "$(zhi completion bash)"
eval "$(zhi completion zsh)"
```

## Global Configuration

```yaml
# ~/.zhi/config.yaml
sharing:
  defaultRegistry: ghcr.io
  marketplace:
    url: https://marketplace.zhi.dev
    apiKey: zhk_abc123...      # for publishing and rating
  registries:
    ghcr.io:
      username: myuser
      # token stored in system keychain or credential helper
    harbor.internal:5000:
      insecure: true           # allow HTTP for internal registries
      mirror: true             # acts as pull-through cache
  verification:
    requireSignatures: false   # set to true for strict mode
    trustedPublishers:         # auto-accept from these publishers
      - zhi-project
      - myorg
  updates:
    checkInterval: 24h         # how often to check for updates
    autoCheck: true            # check on zhi run
    autoInstall: false         # never auto-install without confirmation
```
