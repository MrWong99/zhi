![zhi](assets/banner.png)

# zhi

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/MrWong99/zhi.svg)](https://pkg.go.dev/github.com/MrWong99/zhi)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg)](CODE_OF_CONDUCT.md)

**zhi** is a modern platform for **configuration management** and **provisioning**.
It gives you an extensible plugin system that auto-generates a friendly terminal UI, while keeping all your configs **secure by default**. Think of it as *"Vault meets Terraform, but for your app config"*. 🔐

### 🎯 Core Principles

1. **🛡️ Configuration & Validation** -- structured, validated settings that catch misconfigurations before they reach production.
2. **🔐 Secure Storage** -- encrypted at rest, so confidentiality and compliance come out of the box.
3. **🧩 Modularity** -- an extensible plugin system that supports multiple provisioning backends (Docker Compose, Kubernetes, Helm, and beyond).

---

## ✨ Features

- **🔒 Encrypted configuration at rest** -- store plugin layer supports encryption and key rotation
- **✅ Automatic validation** -- config plugins define validation rules, including cross-value checks
- **🔌 Plugin-based extensibility** -- [gRPC-based plugin system](https://github.com/hashicorp/go-plugin) with four plugin types (config, transform, store, UI) and a [meta-plugin SDK](docs/plugin-development/meta-plugin.md) for composing plugins
- **💻 Auto-generated TUI** -- interactive terminal interface built with [Bubbletea](https://github.com/charmbracelet/bubbletea)
- **🌐 Web UI** -- browser-based editor served on localhost for a richer editing experience
- **🤖 MCP server** -- expose configuration management to LLM clients (Claude Desktop, Claude Code, Cursor) via the [Model Context Protocol](https://modelcontextprotocol.io)
- **📦 Component model** -- group configuration into toggleable bundles with dependencies
- **📄 Template-based exports** -- render configuration to JSON, YAML, TOML, dotenv, or custom templates
- **🚀 Provisioning** -- trigger external commands (Docker Compose, kubectl, Ansible, etc.) with exported configuration
- **📡 Plugin sharing** -- [install, update, and publish](docs/user-guide/sharing-and-registries.md) plugins via OCI registries with signature verification, version pinning, rollback, and binary integrity checks
- **🏪 Marketplace** -- search and rate plugins, verified publisher program, vulnerability advisories
- **🏢 Enterprise mirror** -- [`zhi-mirror`](docs/user-guide/enterprise-mirror.md) provides a local OCI pull-through cache with policy controls, audit logging, and air-gapped export/import

---

## 🚀 Getting Started

### Prerequisites

- Go 1.26+ (only needed when building from source)

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

The binary lands in `bin/`. You're ready to roll! 🎉

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

# Launch an editor
zhi edit
```

### 💻 Interactive TUI Editor

![zhi edit tui](assets/tui.gif)

### 🌐 Web UI Editor

![zhi edit webui](assets/webui.png)

### 🤖 MCP Server (for LLM Clients)

zhi includes built-in MCP (Model Context Protocol) support, allowing LLM clients like Claude Desktop, Claude Code, and Cursor to manage configurations programmatically. Your AI assistant can now tweak your configs -- what a time to be alive!

**Stdio transport** (recommended for local use):

```sh
zhi edit --ui mcp-stdio
```

Configure in Claude Desktop's `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "zhi": {
      "command": "zhi",
      "args": ["edit", "--ui", "mcp-stdio"],
      "cwd": "/path/to/workspace"
    }
  }
}
```

**HTTP transport** (for remote/network access): install the `zhi-ui-mcp-sse` external plugin. See [`examples/zhi-ui-mcp-sse/`](examples/zhi-ui-mcp-sse/).

### 📦 Components

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

### 🔍 Verbose / Debug Logging

Pass `--verbose` to any command to enable debug-level logging to stderr:

```sh
zhi validate --verbose
```

---

## 📚 User Guide

Detailed documentation for using zhi:

- [Getting Started](docs/user-guide/getting-started.md) -- installation and first workspace
- [Workspace Configuration](docs/user-guide/workspace-configuration.md) -- `zhi.yaml` reference
- [CLI Reference](docs/user-guide/cli-reference.md) -- all commands and flags
- [Components](docs/user-guide/components.md) -- grouping and toggling configuration bundles
- [Export and Templates](docs/user-guide/export-and-templates.md) -- template syntax and format helpers
- [Apply](docs/user-guide/apply.md) -- running provisioning commands
- [Plugin Discovery](docs/user-guide/plugin-discovery.md) -- discovering and using external plugins
- [Sharing and Registries](docs/user-guide/sharing-and-registries.md) -- installing, publishing, and updating plugins via OCI registries
- [Marketplace Indexing](docs/user-guide/marketplace-indexing.md) -- adding plugins to the marketplace search index
- [Enterprise Mirror](docs/user-guide/enterprise-mirror.md) -- local OCI mirror for air-gapped environments
- [Web UI](docs/user-guide/web-ui.md) -- browser-based configuration editor

---

## 🔌 Plugin Development

zhi's plugin system supports four types: **config**, **transform**, **store**, and **UI**. Plugins are separate binaries communicating over gRPC -- so they can be written in any language that speaks gRPC.

- [Plugin Development Overview](docs/plugin-development/overview.md) -- shared concepts, binary structure, testing
- [Config Plugin API](docs/plugin-development/config-plugin.md) -- manage configuration values
- [Transform Plugin API](docs/plugin-development/transform-plugin.md) -- mutate trees before display or after save
- [Store Plugin API](docs/plugin-development/store-plugin.md) -- persist and retrieve trees
- [UI Plugin API](docs/plugin-development/ui-plugin.md) -- provide interactive frontends
- [Meta-Plugin SDK](docs/plugin-development/meta-plugin.md) -- launch, delegate, and compose child plugins
- [Structured File Provider](docs/plugin-development/structuredfile-provider.md) -- built-in file-based config provider
- [Java Plugin Development](docs/plugin-development/java-plugin.md) -- build plugins in Java with GraalVM
- [Plugin Scaffolding](docs/plugin-development/scaffolding.md) -- generate a new plugin project with `zhi plugin new`

### 🧪 Examples

The [`examples/`](examples/) directory contains fully working plugins you can build and experiment with. Go ahead, break things -- that's what examples are for! 🔬

| Example | Type | What it demonstrates |
|---------|------|---------------------|
| [zhi-config-pokedex](examples/zhi-config-pokedex/) | Config | Typed values, metadata, validation -- gotta validate 'em all! |
| [zhi-transform-pokedex](examples/zhi-transform-pokedex/) | Transform | Tree mutation, value mapping |
| [zhi-store-json](examples/zhi-store-json/) | Store | File-based persistence |
| [zhi-store-memory](examples/zhi-store-memory/) | Store | Minimal in-memory store |
| [zhi-store-vault](examples/zhi-store-vault/) | Store | HashiCorp Vault KV v2 backend |
| [zhi-store-mirror](examples/zhi-store-mirror/) | Store (meta) | Meta-plugin: mirrors writes to memory + JSON file |
| [zhi-ui-httpapi](examples/zhi-ui-httpapi/) | UI | HTTP/JSON API with SSE streaming |
| [zhi-ui-mcp-sse](examples/zhi-ui-mcp-sse/) | UI | MCP server over HTTP for LLM clients |
| [zhi-ui-webui](examples/zhi-ui-webui/) | UI | Browser-based Web UI |
| [zhi-config-javabean](examples/zhi-config-javabean/) | Config | Java plugin with Bean Validation and GraalVM native-image |

All Go example plugins are published to the GitHub Container Registry on every release and can be installed directly:

```sh
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-memory:v0.0.9
```

See [Sharing and Registries](docs/user-guide/sharing-and-registries.md) for the full list and details.

Build everything locally with:

```sh
make build-all
```

See the [examples README](examples/README.md) for more details.

---

## 🏢 Enterprise Mirror (`zhi-mirror`)

For enterprise and air-gapped environments, `zhi-mirror` provides a local registry mirror with:

- **OCI pull-through cache** -- transparently caches artifacts from upstream registries
- **Marketplace API proxy** -- caches and filters marketplace search results
- **Policy engine** -- control which publishers, artifact types, and plugins are allowed
- **Audit logging** -- structured JSON logs of all pull operations for SIEM integration
- **Pre-population sync** -- scheduled syncing of approved artifacts
- **Air-gapped export/import** -- bundle artifacts for transfer to disconnected networks

```sh
# Start the mirror server
zhi-mirror serve --listen :5050 --upstream-registry ghcr.io

# Export artifacts for air-gapped transfer
zhi-mirror export --artifacts zhi-project/zhi-config-ansible:v1.2.0 --output bundle.tar

# Import into an air-gapped mirror
zhi-mirror import --input bundle.tar
```

Clients point to the mirror by setting `sharing.defaultRegistry` in `~/.zhi/config.yaml` or via the `ZHI_REGISTRY` environment variable. See the [Enterprise Mirror guide](docs/user-guide/enterprise-mirror.md) for the full documentation.

---

## 🤝 Contributing

Contributions are welcome -- whether it's a bug fix, a new plugin idea, or improving documentation. Every contribution helps zhi evolve. ❤️

Please read the [Contributing Guide](CONTRIBUTING.md) to get started. The short version:

```sh
git clone https://github.com/MrWong99/zhi.git
cd zhi
make deps
make check
```

Before opening a pull request, make sure `make check` passes.

---

## 🌍 Community

- [**Code of Conduct**](CODE_OF_CONDUCT.md) -- how we treat each other (tl;dr: be excellent to each other 🤙)
- [**Contributing Guide**](CONTRIBUTING.md) -- setup, workflow, and code style
- [**Security Policy**](SECURITY.md) -- how to report vulnerabilities
- [**Issue Templates**](.github/ISSUE_TEMPLATE/) -- bug reports, feature requests, and plugin ideas

---

## 🀄 The Origin of the Name

<img src="assets/logo.png" alt="zhi logo" width="200" align="right" />

The name **zhi** (pronounced roughly *"jrr"* in Mandarin) carries layered meanings:

- In **Chinese (置)**, *zhi* means *"to place, set, arrange"* -- directly resonating with configuration and provisioning.
- In **Chinese (智)**, *zhi* means *"wisdom, knowledge"* -- emphasizing security, clarity, and correctness.

This multiplicity reflects the project's vision:
to **arrange systems securely**, with **wisdom in design**, while remaining **flexible and modular**.

---

## 📜 License

[MIT](LICENSE) -- Copyright (c) 2025 Lukas Schmidt
