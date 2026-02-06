![zhi](assets/banner.png)

# zhi

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg)](CODE_OF_CONDUCT.md)

**zhi** is a modern platform for **configuration management** and **provisioning**.
It provides an extensible plugin system that automatically generates a user-friendly terminal UI, while keeping all configurations **secure by default**.

Core principles:

1. **Configuration & Validation** -- structured, validated settings that prevent misconfiguration before deployment.
2. **Secure Storage** -- encrypted at rest, ensuring confidentiality and compliance without extra setup.
3. **Modularity** -- built around an extensible plugin system, enabling support for multiple provisioning backends (Docker Compose, Kubernetes, Helm, and beyond).

---

## Features

- **Encrypted configuration at rest** -- store plugin layer supports encryption and key rotation
- **Automatic validation** -- config plugins define validation rules, including cross-value checks
- **Plugin-based extensibility** -- [gRPC-based plugin system](https://github.com/hashicorp/go-plugin) with four plugin types (config, transform, store, UI)
- **Auto-generated TUI** -- interactive terminal interface built with [Bubbletea](https://github.com/charmbracelet/bubbletea)
- **Component model** -- group configuration into toggleable bundles with dependencies
- **Template-based exports** -- render configuration to JSON, YAML, TOML, dotenv, or custom templates
- **Provisioning** -- trigger external commands (Docker Compose, kubectl, Ansible, etc.) with exported configuration
- **Plugin sharing** -- install and publish plugins via OCI registries with signature verification and binary integrity checks

---

## Getting Started

### Prerequisites

- Go 1.24+ (only needed when building from source)

### Installation

**From a release (recommended):**

```sh
# Linux amd64
curl -sSL https://github.com/MrWong99/zhi/releases/latest/download/zhi_linux_amd64.tar.gz | tar xz
sudo mv zhi /usr/local/bin/

# macOS arm64 (Apple Silicon)
curl -sSL https://github.com/MrWong99/zhi/releases/latest/download/zhi_darwin_arm64.tar.gz | tar xz
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

### Quick Start

```sh
# Initialize a new workspace
zhi init

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

### Components

Components group related configuration into named bundles that users can toggle:

```sh
# List components and their status
zhi list components

# Enable a component (auto-enables dependencies)
zhi component enable redis

# Disable a component
zhi component disable monitoring
```

Components defined in `zhi.yaml` control which paths appear in exports and the TUI. See the [Components guide](docs/user-guide/components.md) for details.

### Verbose / Debug Logging

Pass `--verbose` to any command to enable debug-level logging to stderr:

```sh
zhi validate --verbose
```

---

## User Guide

Detailed documentation for using zhi:

- [Getting Started](docs/user-guide/getting-started.md) -- installation and first workspace
- [Workspace Configuration](docs/user-guide/workspace-configuration.md) -- `zhi.yaml` reference
- [CLI Reference](docs/user-guide/cli-reference.md) -- all commands and flags
- [Components](docs/user-guide/components.md) -- grouping and toggling configuration bundles
- [Export and Templates](docs/user-guide/export-and-templates.md) -- template syntax and format helpers
- [Apply](docs/user-guide/apply.md) -- running provisioning commands
- [Plugin Discovery](docs/user-guide/plugin-discovery.md) -- installing and using external plugins

---

## Plugin Development

zhi's plugin system supports four types: **config**, **transform**, **store**, and **UI**. Plugins are separate binaries communicating over gRPC.

- [Plugin Development Overview](docs/plugin-development/overview.md) -- shared concepts, binary structure, testing
- [Config Plugin API](docs/plugin-development/config-plugin.md) -- manage configuration values
- [Transform Plugin API](docs/plugin-development/transform-plugin.md) -- mutate trees before display or after save
- [Store Plugin API](docs/plugin-development/store-plugin.md) -- persist and retrieve trees
- [UI Plugin API](docs/plugin-development/ui-plugin.md) -- provide interactive frontends
- [Structured File Provider](docs/plugin-development/structuredfile-provider.md) -- built-in file-based config provider

### Examples

The [`examples/`](examples/) directory contains fully working plugins you can build and experiment with:

| Example | Type | What it demonstrates |
|---------|------|---------------------|
| [zhi-config-pokedex](examples/zhi-config-pokedex/) | Config | Typed values, metadata, validation |
| [zhi-transform-pokedex](examples/zhi-transform-pokedex/) | Transform | Tree mutation, value mapping |
| [zhi-store-json](examples/zhi-store-json/) | Store | File-based persistence |
| [zhi-store-memory](examples/zhi-store-memory/) | Store | Minimal in-memory store |
| [zhi-ui-httpapi](examples/zhi-ui-httpapi/) | UI | HTTP/JSON API with SSE streaming |

Build everything with:

```sh
make build-all
```

See the [examples README](examples/README.md) for more details.

---

## Contributing

Contributions are welcome -- whether it's a bug fix, a new plugin idea, or
improving documentation. Every contribution helps zhi evolve.

Please read the [Contributing Guide](CONTRIBUTING.md) to get started. The
short version:

```sh
git clone https://github.com/MrWong99/zhi.git
cd zhi
make deps
make check
```

Before opening a pull request, make sure `make check` passes.

---

## Community

- [**Code of Conduct**](CODE_OF_CONDUCT.md) -- how we treat each other
  (tl;dr: be a good Trainer)
- [**Contributing Guide**](CONTRIBUTING.md) -- setup, workflow, and code style
- [**Security Policy**](SECURITY.md) -- how to report vulnerabilities
- [**Issue Templates**](.github/ISSUE_TEMPLATE/) -- bug reports, feature
  requests, and plugin ideas

---

## The Origin of the Name

<img src="assets/logo.png" alt="zhi logo" width="200" align="right" />

The name **zhi** (pronounced roughly *"jrr"* in Mandarin) carries layered meanings:

- In **Chinese (置)**, *zhi* means *"to place, set, arrange"* -- directly resonating with configuration and provisioning.
- In **Chinese (智)**, *zhi* means *"wisdom, knowledge"* -- emphasizing security, clarity, and correctness.

This multiplicity reflects the project's vision:
to **arrange systems securely**, with **wisdom in design**, while remaining **flexible and modular**.

---

## License

[MIT](LICENSE) -- Copyright (c) 2025 Lukas Schmidt
