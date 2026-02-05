# Plan: Plugin & Workspace Sharing System

This directory contains the design plan for zhi's plugin and workspace sharing system — a marketplace/registry that allows users to discover, install, publish, and update plugins and prepared workspaces.

## Documents

| Document | Description |
|---|---|
| [Distribution Format](distribution-format.md) | Evaluation of OCI artifacts vs Docker images vs alternatives |
| [Architecture Overview](architecture.md) | System architecture, components, and data flow |
| [Marketplace Server](marketplace-server.md) | Central marketplace API, database schema, and hosting |
| [CLI Integration](cli-integration.md) | New CLI commands for plugin/workspace management |
| [UI Integration](ui-integration.md) | Marketplace browsing from UI plugins |
| [Security & Trust](security.md) | Signing, verification, trust model, and sandboxing |
| [Local Proxy](local-proxy.md) | Local registry as caching proxy for air-gapped environments |
| [Update Mechanism](update-mechanism.md) | Automatic and manual update workflows |
| [Ratings & Verification](ratings-and-verification.md) | Community ratings and verified publisher program |
| [Implementation Roadmap](roadmap.md) | Phased delivery plan (index) with [per-phase details](roadmap/) |

## Design Principles

1. **Ease of use first** — Installing a plugin should be a single command. Browsing the marketplace should be possible from any UI.
2. **Leverage existing ecosystems** — Use OCI registries rather than building custom artifact storage. Reuse Docker Hub, GHCR, and Harbor infrastructure.
3. **Security by default** — All artifacts are signed and verified. Checksums are enforced. Binary auditing is extended from the existing launcher.
4. **Offline-capable** — Air-gapped environments can mirror the registry locally and operate without internet access.
5. **Language-agnostic** — Plugins written in Go, Java, Python, or any gRPC-capable language are first-class citizens with proper runtime dependency handling.
6. **Incremental adoption** — Each phase delivers standalone value. The system works with or without the central marketplace.
