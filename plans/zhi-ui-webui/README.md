# zhi-ui-webui: Web UI Plugin Plan

A browser-based UI plugin for zhi that serves a full-featured configuration management interface as a local website. Built on **server-side rendering** with Go `html/template`, progressive enhancement via **HTMX**, and minimal TypeScript where necessary.

## Goals

1. **Ease of use** -- zero-install browser experience; open a URL and manage configurations
2. **Security** -- defense-in-depth with CSRF tokens, CSP headers, secure defaults, localhost binding
3. **Fanciness** -- modern, polished UI that feels like a desktop app while being HTML-first

## Design Principles

- **Server-side rendering first**: Go `html/template` renders every page. No client-side routing.
- **Progressive enhancement**: HTMX adds interactivity (partial swaps, live validation) without a JS framework.
- **Forms over fetch**: HTML `<form>` elements with POST for mutations; HTMX for inline updates.
- **TypeScript only when necessary**: Real-time streaming (SSE for apply), code editors, keyboard shortcuts.
- **No build step required**: HTMX and Alpine.js loaded from embedded static assets. TypeScript compiled at build time via `esbuild` and embedded into the Go binary.

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                    Browser (HTML/CSS/JS)                  │
│  ┌─────────┐  ┌──────────┐  ┌────────┐  ┌────────────┐  │
│  │  HTMX   │  │Alpine.js │  │  SSE   │  │ TypeScript │  │
│  │ partials │  │ tooltips │  │ stream │  │  (minimal) │  │
│  └────┬────┘  └────┬─────┘  └───┬────┘  └─────┬──────┘  │
│       │            │            │              │          │
└───────┼────────────┼────────────┼──────────────┼──────────┘
        │ HTTP       │ HTTP       │ SSE          │ HTTP
┌───────┼────────────┼────────────┼──────────────┼──────────┐
│       ▼            ▼            ▼              ▼          │
│  ┌──────────────────────────────────────────────────┐    │
│  │              Go HTTP Server (net/http)            │    │
│  │  ┌──────────┐ ┌──────────┐ ┌────────────────┐   │    │
│  │  │ Handlers │ │Middleware│ │ Template Engine │   │    │
│  │  │ (routes) │ │(security)│ │  (html/template)│   │    │
│  │  └────┬─────┘ └──────────┘ └────────────────┘   │    │
│  │       │                                          │    │
│  │       ▼                                          │    │
│  │  ┌──────────────────────────┐                    │    │
│  │  │  ui.Controller (gRPC)    │                    │    │
│  │  │  LoadTree, SetValue,     │                    │    │
│  │  │  Validate, Save, Export, │                    │    │
│  │  │  Apply, Components,      │                    │    │
│  │  │  Marketplace             │                    │    │
│  │  └──────────────────────────┘                    │    │
│  └──────────────────────────────────────────────────┘    │
│                     zhi-ui-webui plugin                   │
└──────────────────────────────────────────────────────────┘
```

## Technology Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Server | Go `net/http` + `html/template` | Native to zhi ecosystem, zero deps, SSR-first |
| Interactivity | [HTMX](https://htmx.org/) ~14kB | Partial page updates without JS framework |
| Micro-reactivity | [Alpine.js](https://alpinejs.dev/) ~15kB | Tooltips, dropdowns, modals -- declarative in HTML |
| Streaming | Server-Sent Events (SSE) | Native browser API, HTMX SSE extension |
| TypeScript | esbuild (compile at build time) | Code editor enhancements, keyboard shortcut manager |
| CSS | Custom design system with CSS custom properties | Theming (dark/light), no external framework dependency |
| Embedding | Go `embed` directive | Single binary distribution -- all assets compiled in |

## Document Index

- [DESIGN.md](DESIGN.md) -- Design philosophy, visual language, and interaction patterns
- [ARCHITECTURE.md](ARCHITECTURE.md) -- Technical architecture, file structure, and data flow
- [SECURITY.md](SECURITY.md) -- Threat model, security controls, and hardening
- [roadmap/](roadmap/) -- Implementation phases:
  - [Phase 1: Foundation](roadmap/phase-1-foundation.md) -- Server, templates, layout, tree view
  - [Phase 2: Core Interaction](roadmap/phase-2-core-interaction.md) -- Editing, validation, save, components
  - [Phase 3: Export & Apply](roadmap/phase-3-export-and-apply.md) -- Export system, real-time apply streaming
  - [Phase 4: Marketplace & Polish](roadmap/phase-4-marketplace-and-polish.md) -- Marketplace, themes, accessibility
  - [Phase 5: Production Hardening](roadmap/phase-5-production-hardening.md) -- Security hardening, performance, testing
