# Plugin Composition Design

**Status**: Draft
**Date**: 2026-02-18

## Overview

This directory contains design proposals for a **plugin composition** feature
in zhi. The goal is to enable users and plugin developers to combine existing
plugins into more specialized plugins without rewriting them from scratch, even
when the component plugins are written in different programming languages.

## Motivating Examples

### Example 1: Extended Vault Store

A store plugin that wraps the standard `zhi-store-vault` plugin but adds
the ability to create Vault ACL policies and AppRole credentials.
Applications deployed via this zhi stack can then use those credentials
to access secrets stored in Vault.

### Example 2: Merged Config Plugins

Plugin A provides a config tree for application A. Plugin B provides a
config tree for application B. Plugin C combines both so the user can
view and edit configs for both applications in one unified tree.

### Example 3: Store with Backup

A store plugin that writes to a primary Vault store and mirrors every
write to a secondary JSON file store as a backup.

### Example 4: Config with Secret Injection

A config plugin that reads its base tree from a `structuredfile` provider
but overlays secret values from a second config plugin that fetches them
from an external secrets manager at runtime.

## Cross-Language Requirement

Since all zhi plugins communicate over gRPC (via hashicorp/go-plugin),
composition must work regardless of the implementation language of each
child plugin. A composition plugin written in Go must be able to delegate
to a child plugin written in Python or Rust, and vice versa.

## Design Documents

| Document | Approach |
|----------|----------|
| [01-engine-multi-provider.md](01-engine-multi-provider.md) | Engine-level support for multiple providers per slot |
| [02-declarative-composition.md](02-declarative-composition.md) | Composition defined declaratively in `zhi.yaml` |
| [03-meta-plugin.md](03-meta-plugin.md) | Meta-plugin that launches and orchestrates child plugins |
| [04-sdk-composition-helpers.md](04-sdk-composition-helpers.md) | SDK libraries for wrapping and combining plugins in code |
| [05-grpc-proxy.md](05-grpc-proxy.md) | Generic gRPC proxy with routing rules |
| [06-comparison.md](06-comparison.md) | Side-by-side comparison and recommendation |
