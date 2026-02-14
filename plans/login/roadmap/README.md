# Login System Implementation Roadmap

## Phase Overview

| Phase | Name | Description | Dependencies |
|-------|------|-------------|--------------|
| 1 | [Core Session Manager](phase1-core-session.md) | `SessionManager` in `internal/core/` with state machine, expiry tracking, auth error handling | None |
| 2 | [Controller API Extensions](phase2-controller-api.md) | Add auth methods to `Controller` interface, `UIController`, `ControllerAdapter`, gRPC proto | Phase 1 |
| 3 | [WebUI Login](phase3-webui.md) | Login page, auth middleware, logout, error handling for browser UI | Phase 2 |
| 4 | [TUI Login](phase4-tui.md) | Login view with Bubbletea, method selection, credential form, re-auth | Phase 2 |
| 5 | [Integration Testing](phase5-integration.md) | End-to-end validation with Vault store across both UIs | Phases 1-4 |
| 6 | [Browser-Based Auth](phase6-browser-auth.md) | OIDC and other redirect-based auth flows (local callback server vs polling) | Phases 1-5 |

## Dependency Graph

```
Phase 1 ──> Phase 2 ──> Phase 3 (WebUI)
                    ──> Phase 4 (TUI)     ──> Phase 5 ──> Phase 6
```

Phases 3 and 4 can be implemented in parallel after Phase 2 is complete.

## Key Design Documents

- [design.md](../design.md) -- Architecture, session manager, controller API, security considerations
- [proto.md](../proto.md) -- gRPC message definitions for the UI controller service
- [webui.md](../webui.md) -- WebUI routes, templates, auth middleware
- [tui.md](../tui.md) -- TUI login view, Bubbletea model, key bindings
