# Getting Started

This guide walks you through installing zhi and creating your first workspace.

## Prerequisites

- **Go 1.26+** (only needed when building from source)

## Installation

**From a release (recommended):**

Release archives are named `zhi_<version>_<os>_<arch>.tar.gz`. Download the
archive that matches your platform from the
[releases page](https://github.com/MrWong99/zhi/releases/latest), or fetch it
via the GitHub API:

```sh
# Determine the latest version
ZHI_TAG=$(curl -sSL https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')
ZHI_VERSION="${ZHI_TAG#v}"

# Linux amd64
curl -sSL "https://github.com/MrWong99/zhi/releases/download/${ZHI_TAG}/zhi_${ZHI_VERSION}_linux_amd64.tar.gz" | tar xz
sudo mv zhi /usr/local/bin/

# macOS arm64 (Apple Silicon)
curl -sSL "https://github.com/MrWong99/zhi/releases/download/${ZHI_TAG}/zhi_${ZHI_VERSION}_darwin_arm64.tar.gz" | tar xz
sudo mv zhi /usr/local/bin/
```

**From source:**

```sh
go install github.com/MrWong99/zhi/cmd/zhi@latest
```

**Build from repository:**

```sh
git clone https://github.com/MrWong99/zhi.git
cd zhi
make build
```

The binary is placed in `bin/`.

## Creating a Workspace

A workspace is a directory containing a `zhi.yaml` file that declares which providers to use and how configuration is structured.

```sh
# Initialize a new workspace in the current directory
zhi init
```

This creates:

- `zhi.yaml` -- workspace configuration
- `config/` -- starter configuration files
- `templates/` -- sample export templates
- `.zhi/` -- local state (store data, component state)

## Basic Usage

```sh
# View all configuration paths
zhi list paths

# Get a specific value
zhi get database/host

# Set a value
zhi set database/host mydb.example.com

# Validate the configuration
zhi validate

# Export as JSON
zhi export --format json

# Launch the interactive TUI editor
zhi edit
```

## Installing Plugins

Install community or third-party plugins directly from OCI registries:

```sh
# Install from the marketplace
zhi plugin install ansible-config

# Install a specific version from an OCI reference
zhi plugin install oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0

# Search for available plugins
zhi plugin search --type config
```

See [Sharing and Registries](sharing-and-registries.md) for the full guide.

## Setup zhi marketplace and mirror

You can set up a local mirror and marketplace including a HashiCorp Vault storage backend.
There are pre-built zhi workspaces available to do this:

```sh
# Determine the latest released version
ZHI_TAG=$(curl -sSL https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')

# Install the Vault deployment workspace
zhi workspace install "oci://ghcr.io/mrwong99/zhi/zhi-workspace-vault:${ZHI_TAG}" ./vault

# Edit the settings for vault in the webui (set admin credentials), then "Export" -> "Export all" and "Apply" -> "Run"
cd vault && zhi edit && cd ..
# !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
# MAKE SURE TO SAVE THE VAULT UNSEAL KEY(S) IN A SAFE PLACE!!!
# !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

# Now open the Vault UI in your browser http(s)://<api address>/ui/ and enable the "kv" secret engine

# Finally install the zhi infrastructure workspace (mirror + marketplace)
zhi workspace install "oci://ghcr.io/mrwong99/zhi/zhi-workspace-zhi:${ZHI_TAG}" ./mirror
cd mirror
# optionally edit the store options in the ./zhi.yaml and then
zhi edit

# Login using the admin credentials with userpass in the webui,
# make your changes, enable the "Components" you want and then "Export" -> "Export all" and "Apply" -> "Run"
```

## What's Next

- [Workspace Configuration](workspace-configuration.md) -- learn about `zhi.yaml`
- [CLI Reference](cli-reference.md) -- all available commands
- [Components](components.md) -- group and toggle configuration bundles
- [Export and Templates](export-and-templates.md) -- render configuration to files
- [Apply](apply.md) -- run provisioning commands
- [Plugin Discovery](plugin-discovery.md) -- discovering and using external plugins
- [Sharing and Registries](sharing-and-registries.md) -- installing, publishing, and updating plugins via OCI registries
- [Plugin Scaffolding](../plugin-development/scaffolding.md) -- generate a new plugin project with `zhi plugin new`
