---
title: "Fix CLI UX Improvements"
type: fix
status: completed
date: 2026-02-21
---

# Fix CLI UX Improvements

## Overview

Address 5 CLI UX findings discovered during new workspace setup. These range from silent data loss (`zhi set` not persisting) to formatting issues and missing TLS support. Finding 6 (component dependency management) is positive feedback and requires no changes.

## Problem Statement

1. **`zhi set` is in-memory only** — values disappear after the process exits, confusing users who expect persistence
2. **`zhi init` lacks `--bare`** — always scaffolds the Pokedex demo, forcing manual cleanup for real workspaces
3. **Validation output misaligned** — hardcoded `%-20s` column width breaks formatting for paths >20 chars
4. **YAML int vs JSON float64** — `yaml.v3` loads integers as Go `int`, but after a store round-trip they return as `float64`, causing silent value rejection during merge
5. **Vault TLS broken** — `VAULT_CACERT` not read; bare `http.Client{}` with no TLS config forces `SSL_CERT_FILE` workaround. Commit `4e31519` fixed Docker deployment config, not the Go client.

## Cross-Fix Dependencies

```
Fix 4 (numeric types) ← Fix 1 depends on this (set+save+load cycle triggers type mismatch)
Fix 2 (bare init)     ← Fix 1 depends on this (bare init should include store for auto-save)
Fix 3 (validate)      — independent
Fix 5 (vault TLS)     — independent
```

## Implementation Order

1. **Fix 4** — Numeric type normalization (foundational; prevents silent data loss that Fix 1 would amplify)
2. **Fix 3** — Validation output alignment (isolated, low risk)
3. **Fix 5** — Vault TLS support (self-contained)
4. **Fix 2** — `--bare` init flag (needed before Fix 1 so new workspaces have a store)
5. **Fix 1** — `zhi set` persistence (depends on Fix 2 + Fix 4 being stable)

---

## Fix 4: Normalize Numeric Types

### Technical Approach

The root cause is that `yaml.v3` unmarshals `port: 5432` as Go `int`, while `encoding/json` (used by gRPC wire format and all store plugins) unmarshals it as `float64`. When `Engine.LoadTree()` merges stored values back, the strict `reflect.TypeOf` comparison at `internal/core/engine.go:133` rejects the merge silently.

**Canonical type: `int`** (matches YAML/config provider, which is the authoritative source).

**Two-pronged fix:**

#### 4a. Relax the merge comparison in `LoadTree`

Replace the strict `reflect.TypeOf` check with a numeric-compatible comparison.

```go
// internal/core/engine.go:132-137
// Before:
if reflect.TypeOf(storedVal.Val) != reflect.TypeOf(currentVal.Val) {
    Logger().Warn(...)
    continue
}

// After: attempt numeric conversion before rejecting
if reflect.TypeOf(storedVal.Val) != reflect.TypeOf(currentVal.Val) {
    converted, ok := tryNumericConvert(storedVal.Val, reflect.TypeOf(currentVal.Val))
    if !ok {
        Logger().Warn(...)
        continue
    }
    storedVal.Val = converted
}
```

#### 4b. Add `tryNumericConvert` helper

A package-level function in `internal/core/` that handles `int` <-> `int64` <-> `float64` conversion:

- `float64` -> `int`: only if `math.Trunc(f) == f` and value fits in `int` range
- `float64` -> `int64`: same check
- `int64` -> `int`: if value fits
- `int` -> `int64`: always safe
- All other numeric conversions: safe widening

#### 4c. Fix `parseValue` in `set.go`

Change `parseValue` at `internal/cli/set.go:107` to return `int` instead of `int64` for consistency with `yaml.v3`:

```go
// Before:
if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
    return i  // returns int64
}

// After:
if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
    if i >= math.MinInt && i <= math.MaxInt {
        return int(i)  // returns int, matching yaml.v3
    }
    return i  // keep int64 for values that overflow int (32-bit systems)
}
```

### Files to Modify

- `internal/core/engine.go` — relax merge comparison, add `tryNumericConvert`
- `internal/cli/set.go` — change `parseValue` return type for integers

### Tests

- `internal/core/engine_test.go` — test `LoadTree` merge with `int` config + `float64` stored values
- `internal/core/convert_test.go` (new) — unit tests for `tryNumericConvert` covering: `float64(5432)` -> `int`, `float64(3.14)` -> `int` (should fail), `int64(42)` -> `int`, overflow cases
- `internal/cli/set_test.go` — verify `parseValue("5432")` returns `int` not `int64`

---

## Fix 3: Validation Output Alignment

### Technical Approach

Replace the hardcoded `fmt.Fprintf(w, "%-10s%-20s%s\n", ...)` at `internal/cli/validate.go:147` with the existing `fprintTable` helper from `internal/cli/helpers.go:50-86`, which dynamically computes column widths.

#### Changes to `validate.go`

```go
// internal/cli/validate.go:144-148
// Before:
for _, r := range annotated {
    sev := strings.ToUpper(r.Severity.String())
    fmt.Fprintf(w, "%-10s%-20s%s\n", sev, r.Path, r.Message)
}

// After:
if len(annotated) > 0 {
    rows := make([][]string, len(annotated))
    for i, r := range annotated {
        rows[i] = []string{strings.ToUpper(r.Severity.String()), r.Path, r.Message}
    }
    fprintTable(w, []string{"SEVERITY", "PATH", "MESSAGE"}, rows)
}
```

**Design decisions:**
- Header row added (SEVERITY, PATH, MESSAGE) — acceptable since `--json` is the stable machine-readable format
- Skip headers when no results (just print summary)
- Summary line remains separate, printed after the table

### Files to Modify

- `internal/cli/validate.go` — replace hardcoded format with `fprintTable`

### Tests

- `internal/cli/validate_test.go` — update substring checks to account for header row; add test with long paths (>20 chars) to verify alignment

---

## Fix 5: Vault TLS — `VAULT_CACERT` Support

### Technical Approach

The vault client at `pkg/providers/store/vault/client.go:59` creates a bare `http.Client{}`. The `tlsutil.ClientConfig` type already handles CA files, client certs, and building configured HTTP clients — the vault code just doesn't use it.

#### 5a. Add `InsecureSkipVerify` to `tlsutil.ClientConfig`

```go
// internal/tlsutil/tlsutil.go — add field to ClientConfig
type ClientConfig struct {
    CertFile           string
    KeyFile            string
    CAFile             string
    InsecureSkipVerify bool  // new
}

// Update Enabled() to include InsecureSkipVerify
func (c *ClientConfig) Enabled() bool {
    return (c.CertFile != "" && c.KeyFile != "") || c.CAFile != "" || c.InsecureSkipVerify
}

// Update tlsConfig() to set InsecureSkipVerify
```

#### 5b. Add TLS fields to vault `Config`

```go
// pkg/providers/store/vault/vault.go — extend Config
type Config struct {
    Address    string
    Token      string
    Mount      string
    Prefix     string
    Namespace  string
    CACert     string // Path to CA PEM (VAULT_CACERT)
    ClientCert string // Path to client cert PEM (VAULT_CLIENT_CERT)
    ClientKey  string // Path to client key PEM (VAULT_CLIENT_KEY)
    SkipVerify bool   // Disable TLS verification (VAULT_SKIP_VERIFY)
}
```

#### 5c. Read env vars in `DefaultConfig`

```go
// pkg/providers/store/vault/vault.go — extend DefaultConfig
func DefaultConfig() Config {
    addr := os.Getenv("VAULT_ADDR")
    if addr == "" {
        addr = "http://127.0.0.1:8200"
    }
    return Config{
        Address:    addr,
        Token:      os.Getenv("VAULT_TOKEN"),
        Mount:      "secret",
        Prefix:     "zhi",
        Namespace:  os.Getenv("VAULT_NAMESPACE"),
        CACert:     os.Getenv("VAULT_CACERT"),
        ClientCert: os.Getenv("VAULT_CLIENT_CERT"),
        ClientKey:  os.Getenv("VAULT_CLIENT_KEY"),
        SkipVerify: os.Getenv("VAULT_SKIP_VERIFY") == "true" || os.Getenv("VAULT_SKIP_VERIFY") == "1",
    }
}
```

#### 5d. Build TLS-configured HTTP client in `New`

```go
// pkg/providers/store/vault/vault.go — in New()
func New(cfg Config) (*Store, error) {
    // ... existing validation ...

    var httpClient *http.Client

    tlsCfg := tlsutil.ClientConfig{
        CAFile:             cfg.CACert,
        CertFile:           cfg.ClientCert,
        KeyFile:            cfg.ClientKey,
        InsecureSkipVerify: cfg.SkipVerify,
    }

    if tlsCfg.Enabled() {
        if cfg.SkipVerify {
            Logger().Warn("VAULT_SKIP_VERIFY is enabled — TLS certificate verification disabled")
        }
        var err error
        httpClient, err = tlsCfg.HTTPClient(30 * time.Second)
        if err != nil {
            return nil, fmt.Errorf("configuring vault TLS: %w", err)
        }
    } else {
        httpClient = &http.Client{Timeout: 30 * time.Second}
    }

    // Pass httpClient to newVaultClient...
}
```

#### 5e. Update `newVaultClient` to accept `*http.Client`

```go
// pkg/providers/store/vault/client.go
func newVaultClient(addr, token, namespace string, httpClient *http.Client) *vaultClient {
    return &vaultClient{
        addr:       strings.TrimRight(addr, "/"),
        namespace:  namespace,
        token:      token,
        httpClient: httpClient,
    }
}
```

**Precedence order:** `zhi.yaml` store options > env vars > defaults (matches Vault CLI convention).

**TLS errors caught eagerly** at `New()` time — bad CA path or invalid PEM fails immediately with a clear message, not on first request.

### Files to Modify

- `internal/tlsutil/tlsutil.go` — add `InsecureSkipVerify` to `ClientConfig`
- `pkg/providers/store/vault/vault.go` — extend `Config`, `DefaultConfig`, `New`
- `pkg/providers/store/vault/client.go` — accept `*http.Client` parameter in `newVaultClient`

### Tests

- `internal/tlsutil/tlsutil_test.go` — test `InsecureSkipVerify` behavior
- `pkg/providers/store/vault/vault_test.go` — test `DefaultConfig` reads all `VAULT_*` env vars; test `New` with TLS config builds correct HTTP client; test with `httptest.NewTLSServer` for actual TLS verification
- `pkg/providers/store/vault/client_test.go` — verify `newVaultClient` uses the provided HTTP client

---

## Fix 2: `--bare` Init Flag

### Technical Approach

Add a `--bare` flag that creates a minimal workspace with just `zhi.yaml`, an empty `config/` directory, and `.zhi/store/`. No demo content (no Pokedex data, templates, nginx, docker-compose).

#### 2a. Add flag

```go
// internal/cli/init.go
var initBare bool

func init() {
    initCmd.Flags().BoolVar(&initBare, "bare", false, "create a minimal workspace without demo content")
    // ... existing flags ...
}
```

#### 2b. Conditional scaffolding in `runInit`

When `--bare` is set:
1. Create `.zhi/` and `.zhi/store/` (same as now)
2. Write a minimal `zhi.yaml` directly (not from the embedded template) containing only:
   - `version: 1`
   - `config:` section with chosen provider + `directory: ./config`
   - `store:` section with chosen provider + `directory: ./.zhi/store`
3. Create empty `config/` directory
4. Print bare-specific "What's next" guidance

When `--bare` is NOT set, behavior is unchanged (full Pokedex demo).

#### 2c. Bare `zhi.yaml` content

```yaml
version: 1

config:
  provider: {{ .ConfigProvider }}
  options:
    directory: ./config

store:
  provider: {{ .StoreProvider }}
  options:
    directory: ./.zhi/store
```

Note: the current full template does NOT include a `store:` section. The bare template includes one so `zhi set` auto-save (Fix 1) works out of the box.

#### 2d. Bare "What's next" guidance

```
What's next?
  1. Add config files in:         config/
  2. See your configuration:      zhi list
  3. Edit interactively:          zhi edit
  4. Manage components:           zhi component list

Add .zhi/ to your .gitignore to keep internal state out of version control.
```

### Files to Modify

- `internal/cli/init.go` — add `--bare` flag, conditional scaffolding, bare template, bare guidance

### Tests

- `internal/cli/init_test.go` — add `TestInitBareCreatesMinimalWorkspace` (verify only `zhi.yaml`, `config/`, `.zhi/store/` created; verify no Pokedex content); `TestInitBareWithCustomProviders`; `TestInitBareForce`

---

## Fix 1: Make `zhi set` Persist Values

### Technical Approach

Auto-save by default after `zhi set`. Add `--no-save` flag to opt out (for scripting scenarios that want the current in-memory-only behavior).

#### 1a. Add `--no-save` flag

```go
// internal/cli/set.go
var setNoSave bool

func init() {
    setCmd.Flags().BoolVar(&setNoSave, "no-save", false, "set value in-memory only without persisting to store")
    // ... existing flags ...
}
```

#### 1b. Save flow in `runSet`

After setting the value, if `--no-save` is not set:

1. Load the full tree: `eng.LoadTree(ctx)` (merges config + stored values)
2. Run validation if `--validate` is set — abort save on blocking errors
3. Apply save transforms: `eng.TransformForSave(ctx, tree)`
4. Save: `eng.SaveTree(ctx, tree)`
5. Print confirmation with save status

```go
func runSet(cmd *cobra.Command, args []string) error {
    // ... existing path/value parsing and SetValue ...

    if setNoSave {
        fmt.Fprintf(w, "Set %s = %v (in-memory only)\n", path, val)
        return nil
    }

    // Load full tree for save
    tree, err := eng.LoadTree(ctx)
    if err != nil {
        return fmt.Errorf("loading tree: %w", err)
    }

    // Validate if requested — abort save on blocking
    if setValidate {
        results, err := eng.Validate(ctx, tree)
        if err != nil {
            return fmt.Errorf("validation error: %w", err)
        }
        hasBlocking := false
        for _, r := range results {
            fmt.Fprintf(w, "  %s: %s\n", r.Severity, r.Message)
            if r.Severity == config.Blocking {
                hasBlocking = true
            }
        }
        if hasBlocking {
            return fmt.Errorf("validation failed with blocking errors, value not saved")
        }
    }

    // Transform and save
    if err := eng.TransformForSave(ctx, tree); err != nil {
        return fmt.Errorf("applying save transforms: %w", err)
    }
    if err := eng.SaveTree(ctx, tree); err != nil {
        // Graceful handling for no-store workspaces
        Logger().Warn("value set but not persisted", "error", err)
        fmt.Fprintf(w, "Set %s = %v (not persisted: %v)\n", path, val, err)
        return nil
    }

    fmt.Fprintf(w, "Set %s = %v (saved)\n", path, val)
    return nil
}
```

**Key design decisions:**
- **No store configured?** Warn and succeed (value is set in-memory, not persisted). This handles workspaces without a store section gracefully.
- **Blocking validation with `--validate`?** Abort the save, return non-zero exit code.
- **Transform before save?** Yes, matching TUI behavior for consistency (encryption, masking, etc.).
- **Confirmation message** distinguishes saved vs in-memory-only vs failed-to-save.

### Files to Modify

- `internal/cli/set.go` — add `--no-save` flag, save flow with transform + graceful error handling

### Tests

- `internal/cli/set_test.go` — test auto-save flow (mock store, verify `PutValues` called); test `--no-save` flag (verify `PutValues` NOT called); test no-store workspace (verify warning, no error); test `--validate` with blocking error (verify save aborted)

---

## Acceptance Criteria

### Functional Requirements

- [x] `zhi set database/port 5432` persists the value to the store (retrievable after restart)
- [x] `zhi set database/port 5432 --no-save` sets in-memory only (current behavior)
- [x] `zhi set` in a workspace with no store configured warns but does not error
- [x] `zhi init --bare` creates minimal workspace: `zhi.yaml`, `config/`, `.zhi/store/`
- [x] `zhi init --bare` does not create any Pokedex/demo content
- [x] `zhi init --bare` includes a store section in `zhi.yaml`
- [x] `zhi validate` output is properly aligned regardless of path length
- [x] `zhi validate --json` output is unchanged
- [x] Integer values survive a YAML load -> store save -> store load cycle without type mismatch
- [x] `parseValue("5432")` returns `int` (not `int64`)
- [x] `VAULT_CACERT` env var is respected for custom CA certificates
- [x] `VAULT_CLIENT_CERT` + `VAULT_CLIENT_KEY` enable mTLS
- [x] `VAULT_SKIP_VERIFY=true` disables TLS verification with a warning
- [x] TLS configuration errors fail eagerly at `New()` with clear messages

### Non-Functional Requirements

- [x] No breaking changes to `--json` output formats
- [x] All existing tests continue to pass
- [x] New code has test coverage for happy path and error cases

### Quality Gates

- [x] `make check` passes (fmt + vet + lint + test)
- [ ] `make test-cover` shows no regression in coverage

## References

### Internal References

- Engine set/save flow: `internal/core/engine.go:203-228`
- Init scaffolding: `internal/cli/init.go:88-161`
- Validation formatting: `internal/cli/validate.go:147`
- Dynamic table helper: `internal/cli/helpers.go:50-86`
- Merge type check: `internal/core/engine.go:132-137`
- Vault client: `pkg/providers/store/vault/client.go:54-61`
- Vault config: `pkg/providers/store/vault/vault.go:37-103`
- TLS client utilities: `internal/tlsutil/tlsutil.go:118-183`
- parseValue: `internal/cli/set.go:89-118`
- TUI save path: `internal/ui/tui/app.go:334`

### Related Commits

- `4e31519` — "workspace(vault): fix TLS management" — fixed Docker deployment config, not Go client
