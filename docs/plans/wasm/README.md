# WebAssembly Plugin Support — Design Plan

This directory contains the detailed design plan for adding WebAssembly (WASM)
plugin support to zhi. WASM plugins provide a **secure, sandboxed, portable**
alternative to the current native-binary gRPC plugin model.

## Goals

1. **Security by default** — WASM plugins run in a memory-safe sandbox with
   explicit capability grants. Unlike native plugins (which can execute
   arbitrary syscalls), a WASM plugin can only access resources the host
   explicitly provides.

2. **Portability** — A single `.wasm` binary runs on any OS/architecture
   without cross-compilation. Plugin authors build once; users install
   anywhere.

3. **Compatibility** — WASM plugins implement the same `config.Plugin`,
   `transform.Plugin`, `store.Plugin`, and `ui.Plugin` interfaces as native
   plugins. The engine and workspace configuration treat them identically.

4. **Incremental adoption** — WASM is a new plugin *format*, not a
   replacement. Native gRPC plugins continue to work unchanged. The two
   models coexist, and plugin authors choose whichever suits them.

## Non-Goals

- Replacing hashicorp/go-plugin or the gRPC transport for native plugins.
- Supporting browser-based WASM execution (js/wasm target).
- Achieving performance parity with native plugins for CPU-heavy workloads.
- Requiring WASM for any existing plugin.

## Minimum Viable Scope

The bare minimum is full support for **config** and **transform** WASM
plugins. These plugin types have simple, stateless interfaces that map
cleanly to WASM function imports/exports.

**Store** plugins are stretch-goal for the initial release. They require
filesystem and/or network access that must be mediated through host
functions, but the design accounts for them from the start.

**UI** plugins are the most ambitious target. WASM UI plugins would need the
bidirectional Controller callback, which is achievable via host functions but
adds complexity. TTY-based UIs remain impossible in WASM (no terminal
access), but HTTP-based UIs (like the existing `zhi-ui-http` example) are a
realistic target.

## Document Index

| Document | Description |
|----------|-------------|
| [01-runtime-evaluation.md](01-runtime-evaluation.md) | Evaluation of WASM runtimes: wazero, wasmtime-go, Extism, knqyf263/go-plugin |
| [02-architecture.md](02-architecture.md) | How WASM plugins integrate into zhi's existing plugin architecture |
| [03-host-functions-abi.md](03-host-functions-abi.md) | Host function ABI design for each plugin type |
| [04-security-model.md](04-security-model.md) | Capability-based sandboxing and permission model |
| [05-plugin-development.md](05-plugin-development.md) | Plugin Development Kit: supported languages, build toolchains, SDK |
| [06-implementation-phases.md](06-implementation-phases.md) | Phased implementation plan with milestones |

## Key Design Decisions (Summary)

These are elaborated in the individual documents but summarized here for
quick reference:

1. **Runtime: wazero** — Pure Go, zero CGO, zero dependencies. Aligns with
   zhi's `CGO_ENABLED=0` static-linking policy and cross-compilation story.
   See [01-runtime-evaluation.md](01-runtime-evaluation.md).

2. **Communication: host functions, not gRPC** — WASM plugins communicate
   via imported/exported functions with JSON-serialized payloads, not over
   gRPC. This eliminates the subprocess + network overhead entirely. See
   [03-host-functions-abi.md](03-host-functions-abi.md).

3. **Discovery: `.wasm` file extension** — WASM plugins are discovered
   alongside native plugins using the same naming conventions but with a
   `.wasm` suffix (e.g., `zhi-config-pokedex.wasm`). See
   [02-architecture.md](02-architecture.md).

4. **Security: opt-in capabilities** — WASM plugins get no filesystem,
   network, or environment access by default. Capabilities are granted
   explicitly in `zhi.yaml` or `zhi-plugin.yaml`. See
   [04-security-model.md](04-security-model.md).

5. **SDK: Go-first, multi-language possible** — The initial Plugin
   Development Kit targets Go (`GOOS=wasip1`). The ABI is language-agnostic,
   so Rust, C, and other languages can produce compatible `.wasm` files. See
   [05-plugin-development.md](05-plugin-development.md).
