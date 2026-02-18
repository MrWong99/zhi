# Plugin Composition Design

**Status**: Active
**Date**: 2026-02-18

## Overview

This directory contains the design for **plugin composition** in zhi —
enabling plugin developers to combine existing plugins into more
specialized plugins without rewriting them from scratch, even when the
component plugins are written in different programming languages.

The design focuses on two complementary pieces:

1. **Meta-Plugin Pattern** — a composition plugin that launches and
   orchestrates child plugins as sub-processes, appearing as a single
   standard plugin to the engine.
2. **SDK Composition Helpers** — library code that eliminates boilerplate
   when building meta-plugins (delegating base types, merge/mirror/overlay
   helpers, plugin launcher).

## Motivating Examples

### Example 1: Extended Vault Store

A store plugin that wraps `zhi-store-vault` but adds automatic Vault ACL
policy and AppRole credential management.

### Example 2: Merged Config Plugins

Plugin A provides config for application A, plugin B for application B.
Plugin C combines both under distinct path prefixes in one unified tree.

### Example 3: Store with Backup

A store writes to a primary Vault store and mirrors every write to a
secondary JSON file store as a backup.

### Example 4: Config with Secret Injection

A config plugin reads its base tree from `structuredfile` but overlays
secret values from a second config plugin backed by a secrets manager.

## Cross-Language Requirement

Since all zhi plugins communicate over gRPC (via hashicorp/go-plugin),
composition works regardless of child plugin implementation language. A
Go meta-plugin can orchestrate children written in Python, Rust, or Java.

## Design Documents

| Document | Description |
|----------|-------------|
| [01-meta-plugin.md](01-meta-plugin.md) | Meta-plugin architecture and pattern |
| [02-sdk-composition-helpers.md](02-sdk-composition-helpers.md) | SDK library design for composition helpers |

## Implementation Plan

The full implementation plan with roadmap is at:
[`docs/plans/2026-02-18-feat-plugin-composition-meta-plugin-sdk-plan.md`](../../docs/plans/2026-02-18-feat-plugin-composition-meta-plugin-sdk-plan.md)
