# 01 — WASM Runtime Evaluation

This document evaluates the available WebAssembly runtimes for embedding in
zhi's Go host process. The runtime is the core dependency: it loads `.wasm`
modules, executes them, and mediates all host↔plugin communication.

## Candidates

Four options were evaluated:

1. **wazero** — Pure Go WASM runtime (tetratelabs/wazero)
2. **wasmtime-go** — Go bindings to Wasmtime via CGO (bytecodealliance/wasmtime-go)
3. **Extism** — High-level plugin framework built on wazero (extism/go-sdk)
4. **knqyf263/go-plugin** — Protobuf-based Go plugin system over wazero

---

## 1. wazero

**Repository:** https://github.com/tetratelabs/wazero
**License:** Apache-2.0
**Latest release:** v1.9.0 (December 2025)

### Overview

wazero is a WebAssembly Core Specification 1.0 and 2.0 compliant runtime
written entirely in Go with zero external dependencies. It does not use CGO.

### Strengths

- **Zero dependencies** — No C compiler, no shared libraries, no libc. This
  directly aligns with zhi's `CGO_ENABLED=0` cross-compilation policy.
- **Two execution modes** — An interpreter (works on all Go targets) and an
  AOT compiler (generates native code for amd64/arm64). The compiler is used
  by default when available.
- **Broad OS/arch support** — Linux, macOS, Windows, FreeBSD, NetBSD,
  OpenBSD, DragonFly BSD, illumos, Solaris. Matches zhi's cross-compilation
  matrix (linux/darwin × amd64/arm64) and then some.
- **Stable API** — SemVer-committed since v1.0 (March 2023). No breaking
  changes without a major version bump.
- **Small binary footprint** — Adds ~5.5 MB to the host binary.
- **WASI Preview 1** — Bundled `wasi_snapshot_preview1` implementation with
  configurable filesystem (`FSConfig`), clocks, args, env, and stdio.
- **Host functions** — Rich `HostModuleBuilder` API for exporting Go
  functions to WASM modules. Functions can use Go closures with full access
  to host state.
- **Concurrency-safe** — Thread-safe module instantiation. `CompiledModule`
  can be reused across goroutines.
- **Active ecosystem** — Imported by 589+ Go packages. Used by Trivy,
  Dapr, Mosn, and others in production.

### Limitations

- **No WASI Preview 2 / Component Model** — wazero targets
  `wasi_snapshot_preview1` only. There is no current plan for P2 support.
  This means no WIT-based typed interfaces or component composition at the
  WASI level.
- **Filesystem sandbox escape** — The current WASI filesystem implementation
  does not fully prevent directory traversal via relative paths (e.g.,
  `../../`). This is documented and tracked. For zhi, this is mitigated by
  not granting filesystem access by default.
- **No network sockets** — `sock_recv`/`sock_send` from WASI P1 are not
  implemented. Network access must be provided through custom host functions.
- **Performance** — Slower than Wasmtime for CPU-bound workloads (no
  Cranelift/LLVM backend). Adequate for I/O-bound plugin workloads like
  config/transform/store operations.
- **Single-threaded WASM execution** — WASM is inherently single-threaded.
  A host function call from the WASM module blocks the module's execution
  until the host returns.

### Verdict for zhi

**Strong fit.** wazero's zero-dependency, pure-Go nature is a direct match
for zhi's build philosophy. The lack of WASI P2 is not a blocker because
zhi's plugin ABI will be defined via custom host functions anyway — not
through WASI system interfaces.

---

## 2. wasmtime-go

**Repository:** https://github.com/bytecodealliance/wasmtime-go
**License:** Apache-2.0

### Overview

Go bindings to the Wasmtime runtime (written in Rust), exposed via a C API
and consumed through CGO.

### Strengths

- **Best-in-class performance** — Cranelift AOT compiler produces
  near-native execution speed. Matters for CPU-bound workloads.
- **WASI Preview 2 + Component Model** — Leading implementation of the
  latest WASI standards. Supports WIT interfaces and component composition.
- **Industry backing** — Maintained by the Bytecode Alliance (Fastly,
  Mozilla, Intel, etc.).
- **Full WASI coverage** — Sockets, filesystem, clocks, random — all
  specified WASI P1 functions implemented plus P2 additions.

### Limitations

- **Requires CGO** — This is the fundamental disqualifier for zhi. The
  project builds with `CGO_ENABLED=0` across all targets. Introducing CGO
  would break cross-compilation, increase binary size by ~38 MB, require
  a C toolchain in CI, and add platform-specific shared library concerns.
- **Complex build chain** — Cross-compiling for linux/darwin × amd64/arm64
  with CGO requires per-target C toolchains (gcc-aarch64-linux-gnu, etc.).
- **Larger binaries** — ~38 MB footprint (full Wasmtime engine).
- **Less Go-idiomatic API** — The Go API wraps C types, leading to
  less natural error handling and memory management patterns.

### Verdict for zhi

**Disqualified.** The CGO requirement is a hard conflict with zhi's static
linking policy. If this constraint were relaxed in the future, wasmtime-go
would be worth revisiting for its P2 support and performance.

---

## 3. Extism

**Repository:** https://github.com/extism/go-sdk
**License:** BSD-3-Clause
**Latest release:** v1.7.0 (March 2025)

### Overview

Extism is a high-level WebAssembly plugin framework that provides a
standardized ABI for host↔plugin communication. The Go SDK is built on
wazero (since v1.0, the Go SDK was rewritten from C bindings to pure
wazero).

### Strengths

- **Pure Go (wazero underneath)** — No CGO. Inherits wazero's
  cross-compilation story.
- **Batteries-included plugin ABI** — Provides memory management (alloc/free),
  input/output buffers, persistent module-scope variables, host function
  linking, and host-controlled HTTP without WASI.
- **Multi-language PDKs** — Plugin Development Kits exist for Go, Rust,
  C, JavaScript, Python, Ruby, and more. Plugin authors write in their
  preferred language.
- **CompiledPlugin for concurrency** — Thread-safe compiled module that can
  produce instances per-call.
- **Runtime limiters** — Built-in execution timeouts and fuel-based
  instruction counting.
- **Production adoption** — Used by Navidrome (2026), Zed IDE, and others.

### Limitations

- **Opinionated ABI** — Extism defines its own calling convention
  (input→function→output via shared memory). This is convenient but adds
  a layer between zhi's interfaces and the WASM module. Mapping zhi's
  multi-method plugin interfaces (e.g., store.Plugin with 20+ methods) onto
  Extism's single-function-call model requires multiplexing.
- **Additional dependency** — Adds Extism as a dependency on top of wazero.
  Given zhi already has a well-defined plugin ABI (proto-based), the added
  abstraction may not justify the extra dependency.
- **Less control** — Extism manages the WASM module lifecycle. For advanced
  scenarios (multiple instances, custom memory layouts, fine-grained
  capability control), direct wazero gives more flexibility.
- **Host function limitations** — While Extism supports host functions,
  they must conform to Extism's memory model. Complex bidirectional protocols
  (like the UI Controller callback) would be awkward.

### Verdict for zhi

**Viable but unnecessary.** Extism is excellent for projects that want a
turnkey plugin system. zhi already has a well-structured plugin architecture
with defined interfaces, gRPC proto definitions, and a registry system.
Using wazero directly gives us more control over the ABI and avoids adding
a framework dependency for functionality we can implement ourselves.

Extism could be reconsidered if multi-language PDK support becomes a high
priority — their PDKs are polished and well-maintained.

---

## 4. knqyf263/go-plugin

**Repository:** https://github.com/knqyf263/go-plugin
**License:** Apache-2.0
**Latest release:** v0.8.0 (March 2025)

### Overview

A Go plugin system built on wazero that auto-generates host and plugin SDKs
from Protocol Buffers definitions. Inspired by hashicorp/go-plugin but
communicates via in-memory WASM function calls instead of gRPC.

### Strengths

- **Proto-based code generation** — This is directly relevant to zhi, which
  already defines its plugin interfaces in `.proto` files. In theory,
  go-plugin could generate the WASM host/plugin stubs from zhi's existing
  proto definitions.
- **Familiar model** — The generated code produces Go interfaces on both
  sides, similar to hashicorp/go-plugin's dispensing model.
- **Pure Go (wazero underneath)** — No CGO.
- **Host functions via proto** — Define host-provided services as proto
  services with a `// go:plugin type=host` annotation.

### Limitations

- **Immature** — v0.8.0, pre-1.0. API may change. Significantly less
  adoption than wazero or Extism.
- **Code generation coupling** — Requires a custom `protoc-gen-go-plugin`
  generator. This adds build toolchain complexity and creates a dependency
  on the project's specific code generation approach.
- **Opaque runtime control** — Like Extism, it wraps wazero and manages
  module lifecycle. Less flexibility for custom sandboxing and capability
  grants.
- **Proto compatibility** — Uses a customized protobuf-go and vtprotobuf
  fork. Unclear how well this interacts with zhi's existing proto
  generation pipeline (`make proto`).
- **Limited documentation** — Fewer examples and less community support
  than wazero or Extism.

### Verdict for zhi

**Interesting but too immature and opaque.** The proto-based approach is
appealing given zhi's existing proto definitions, but the pre-1.0 status,
custom code generation, and limited flexibility make it risky. If the
project matures and stabilizes, it could be a compelling option for
auto-generating the WASM ABI from zhi's protos.

---

## Recommendation

### Primary: wazero (direct)

Use **wazero directly** as the WASM runtime, without an intermediate
framework.

**Rationale:**

| Criterion | wazero | wasmtime-go | Extism | go-plugin |
|-----------|--------|-------------|--------|-----------|
| No CGO required | Yes | **No** | Yes | Yes |
| API stability | Stable (v1.0+) | Stable | Stable | Pre-1.0 |
| Binary size impact | ~5.5 MB | ~38 MB | ~5.5 MB | ~5.5 MB |
| Control over ABI | **Full** | Full | Limited | Limited |
| Control over sandbox | **Full** | Full | Limited | Limited |
| Cross-compilation | Trivial | Complex | Trivial | Trivial |
| WASI P2 support | No | **Yes** | No | No |
| Go ecosystem fit | **Native** | CGO wrapper | Framework | Framework |

wazero provides the right level of abstraction: low enough to define a
custom ABI that maps cleanly to zhi's plugin interfaces, and high enough
to avoid raw WASM instruction manipulation. The lack of WASI P2 is
irrelevant because zhi's host functions define the plugin contract, not
WASI system interfaces.

### Future consideration: Extism

If demand grows for plugins written in Rust, C, JavaScript, or other
languages, Extism's multi-language PDKs become attractive. At that point,
Extism could be adopted as an *optional alternative runtime* alongside
direct wazero, since both use wazero under the hood and can coexist.

### WASI P2/P3 outlook

WASI Preview 2 (component model) stabilized in late 2024 but wazero has
no plans to implement it. WASI 0.3 (async) is expected around mid-2026,
and Go's `GOOS=wasip3` proposal would make goroutine-based plugins viable.
This is worth monitoring but does not block the initial implementation.
When wazero or the Go ecosystem adds P2/P3 support, the ABI can be
evolved to take advantage of typed interfaces (WIT) while maintaining
backward compatibility with the P1-based ABI.
