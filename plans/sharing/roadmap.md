# Implementation Roadmap

Phased delivery plan where each phase delivers standalone value. Later phases build on earlier ones but are not required for the system to be useful.

Each phase has its own detailed document in the [`roadmap/`](roadmap/) subfolder.

## Phases

| Phase | Name | Goal | Prerequisites |
|---|---|---|---|
| [1](roadmap/phase-1-foundation.md) | **Foundation** | Users can install plugins from OCI registries via CLI | None |
| [2](roadmap/phase-2-publishing.md) | **Publishing** | Plugin authors can publish; workspaces can be shared | Phase 1 |
| [3](roadmap/phase-3-security.md) | **Security** | Artifacts are signed and verified; binary integrity enforced | Phases 1, 2 |
| [4](roadmap/phase-4-discovery.md) | **Discovery** | Marketplace API for search and short-name resolution | Phases 1, 2 |
| [5](roadmap/phase-5-community.md) | **Community** | Ratings, verified publishers, vulnerability advisories | Phase 4 |
| [6](roadmap/phase-6-ui-integration.md) | **UI Integration** | Marketplace browsable from TUI, Web, and API UIs | Phase 4 |
| [7](roadmap/phase-7-enterprise.md) | **Enterprise** | Local proxy/mirror with policy controls for air-gapped use | Phases 1, 4 |
| [8](roadmap/phase-8-updates.md) | **Updates** | Automatic update detection, install, and rollback | Phases 1, 4 |
| [--](roadmap/future-ideas.md) | **Future Ideas** | Plugin templates, WASM, composition, federation, and more | Various |

## Dependency Graph

```
Phase 1: Foundation
  ├──▶ Phase 2: Publishing
  │      └──▶ Phase 3: Security
  ├──▶ Phase 4: Discovery
  │      ├──▶ Phase 5: Community
  │      ├──▶ Phase 6: UI Integration
  │      ├──▶ Phase 7: Enterprise
  │      └──▶ Phase 8: Updates
  └──▶ Phase 7: Enterprise (also depends on Phase 1 directly)
```

Phases 5, 6, 7, and 8 can be developed in parallel once Phase 4 is complete.

## Design Documents

Each phase references the relevant design documents for detailed specifications:

| Document | Referenced by Phases |
|---|---|
| [Distribution Format](distribution-format.md) | 1, 2, 3 |
| [Architecture Overview](architecture.md) | 1, 2, 4, 6 |
| [Marketplace Server](marketplace-server.md) | 4, 5 |
| [CLI Integration](cli-integration.md) | 1, 2, 3, 4, 7, 8 |
| [UI Integration](ui-integration.md) | 6 |
| [Security & Trust](security.md) | 3, 5, 7 |
| [Local Proxy](local-proxy.md) | 7 |
| [Update Mechanism](update-mechanism.md) | 2, 5, 8 |
| [Ratings & Verification](ratings-and-verification.md) | 5, 6, 8 |
