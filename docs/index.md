---
hide:
  - navigation
---

# zhi

<p align="center">
  <img src="assets/banner.png" alt="zhi banner" />
</p>

**zhi** is a modern platform for **configuration management** and **provisioning**.
It gives you an extensible plugin system that auto-generates a friendly terminal UI, while keeping all your configs **secure by default**. Think of it as *"Vault meets Terraform, but for your app config"*.

## Core Principles

1. **Configuration & Validation** -- structured, validated settings that catch misconfigurations before they reach production.
2. **Secure Storage** -- encrypted at rest, so confidentiality and compliance come out of the box.
3. **Modularity** -- an extensible plugin system that supports multiple provisioning backends (Docker Compose, Kubernetes, Helm, and beyond).

## Features

- **Encrypted configuration at rest** -- store plugin layer supports encryption and key rotation
- **Automatic validation** -- config plugins define validation rules, including cross-value checks
- **Plugin-based extensibility** -- [gRPC-based plugin system](https://github.com/hashicorp/go-plugin) with four plugin types (config, transform, store, UI) and a [meta-plugin SDK](plugin-development/meta-plugin.md) for composing plugins
- **Auto-generated TUI** -- interactive terminal interface built with [Bubbletea](https://github.com/charmbracelet/bubbletea)
- **Web UI** -- browser-based editor served on localhost for a richer editing experience
- **MCP server** -- expose configuration management to LLM clients (Claude Desktop, Claude Code, Cursor) via the [Model Context Protocol](https://modelcontextprotocol.io)
- **Component model** -- group configuration into toggleable bundles with dependencies
- **Template-based exports** -- render configuration to JSON, YAML, TOML, dotenv, or custom templates
- **Provisioning** -- trigger external commands (Docker Compose, kubectl, Ansible, etc.) with exported configuration
- **Plugin sharing** -- [install, update, and publish](user-guide/sharing-and-registries.md) plugins via OCI registries with signature verification, version pinning, rollback, and binary integrity checks
- **Marketplace** -- search and rate plugins, verified publisher program, vulnerability advisories
- **Enterprise mirror** -- [`zhi-mirror`](user-guide/enterprise-mirror.md) provides a local OCI pull-through cache with policy controls, audit logging, and air-gapped export/import

## Quick Start

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

See the [Getting Started](user-guide/getting-started.md) guide for installation and a full walkthrough.

## The Name

<img src="assets/logo.png" alt="zhi logo" width="200" align="right" />

The name **zhi** (pronounced roughly *"jrr"* in Mandarin) carries layered meanings:

- In **Chinese (置)**, *zhi* means *"to place, set, arrange"* -- directly resonating with configuration and provisioning.
- In **Chinese (智)**, *zhi* means *"wisdom, knowledge"* -- emphasizing security, clarity, and correctness.
