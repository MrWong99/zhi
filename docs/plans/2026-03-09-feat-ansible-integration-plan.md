---
title: "feat: Ansible Integration"
type: feat
status: active
date: 2026-03-09
---

# feat: Ansible Integration

## Enhancement Summary

**Deepened on:** 2026-03-09
**Agents used:** Architecture strategist, security sentinel, performance oracle, code simplicity reviewer, pattern recognition specialist, config plugin skill checker, Ansible best practices researcher

### Key Improvements from Research
1. Prerequisite refactoring: extract shared `ExpandTemplates` before implementing iterate (eliminates duplication between CLI and UIController)
2. Performance: materialize child trees instead of using `prefixTree` wrappers to avoid O(N*M) scans
3. Security: confine `ansible.source_file` write-back to project root directory; validate `output-pattern` at workspace load time
4. Simplification: drop label registration (doesn't work across process boundary), reduce v1 validation to 2 core rules, limit nested YAML to flat var files
5. Template contract: use `IterateData{Key, Value}` with `renderTemplateFile` accepting `any` data — cleanest approach despite breaking dot-is-TreeData convention
6. Missing files identified: `pkg/zhiplugin/ui/ui.go` ExportTemplate needs new fields too

---

## Overview

Integrate Ansible with zhi through four independent, incrementally adoptable layers. Each layer builds on existing extension points — no new plugin types. The integration makes zhi a first-class configuration source for Ansible without reimplementing Ansible itself.

**Implementation order:** Layer 1 (Templates) → Layer 2 (Multi-file Export) → Layer 3 (Config Plugin) → Layer 4 (Pre-checks)

**Design document:** `docs/design/ansible-integration.md`

## Key Decisions from Brainstorm

- Use Ansible-native underscore naming (`ansible_host`, `ansible_port`) — zhi's path regex allows underscores
- Multi-file export iterates over direct children only, no deeper nesting
- No orphan file cleanup — consistent with existing export behavior
- Config plugin v1 scoped to YAML inventory only
- Config plugin supports `Set()` from day one using `ansible.source_file` metadata for write-back
- Pre-checks are hard stop on failure — users create separate apply targets for with/without
- Output parsing (Layer 3b from design doc) is deferred indefinitely

---

## Phase 0: Prerequisite Refactoring

**Goal:** Extract the ExportTemplate-to-ExportRunConfig conversion into a shared function before implementing iterate.

**Why:** This conversion is currently duplicated between `internal/cli/export.go:115-138` and `internal/ui/driver.go:253-278`. Adding iterate awareness to both would compound the duplication. Extract once, then iterate expansion becomes a natural extension.

### Tasks

- [x] Create `core.ExpandTemplates(templates []ExportTemplate, td *TreeData, wsDir string) []ExportRunConfig` in `internal/core/export.go`
- [x] Refactor `internal/cli/export.go:115-138` to call `core.ExpandTemplates`
- [x] Refactor `internal/ui/driver.go:253-278` (`UIController.ExportAll`) to call `core.ExpandTemplates`
- [x] Verify existing export tests still pass

### Acceptance Criteria

- [x] Single conversion function used by both CLI and UI code paths
- [x] No behavior change — pure refactoring
- [x] All existing export tests pass

---

## Phase 1: Workspace Template

**Goal:** Publishable workspace template providing ready-made export templates for Ansible artifacts.

**Scope:** Template files only — zero code changes in zhi core.

### Tasks

- [x] Create workspace template directory structure under `examples/zhi-workspace-ansible-infra/`
- [x] Create `zhi.yaml` with config provider, component definitions, export templates, and apply targets
- [x] Create `templates/inventory.yml.tmpl` — YAML-format static inventory from `hosts/` tree
- [x] Create `templates/group-vars.yml.tmpl` — per-group variable file from `groups/<name>/vars/`
- [x] Create `templates/host-vars.yml.tmpl` — per-host variable file from `hosts/<name>/vars/`
- [x] Create `templates/ansible-cfg.tmpl` — ansible.cfg from `ansible/config/` tree prefix
- [x] Create example tree data files (YAML) with sample hosts, groups, and variables using underscore naming
- [x] Create `zhi-workspace.yaml` manifest for workspace publishing
- [ ] Add `README.md` explaining the tree convention and usage
- [ ] Test: manually verify export produces valid Ansible inventory and variable files

### Research Insights (Ansible Best Practices)

**YAML inventory structure:** The root key MUST be `all`. Groups go under `children`, hosts under `hosts`, variables under `vars`. The `inventory.yml.tmpl` template must generate this exact structure:

```yaml
all:
  hosts:
    ungrouped_host:
      ansible_host: "..."
  children:
    webservers:
      hosts:
        web1:
          ansible_host: "10.0.1.10"
      vars:
        http_port: "8080"
```

**group_vars files must be flat YAML** — key-value pairs at the top level, NOT nested under the group name.

**Group name restrictions:** Ansible requires group names to match `[a-zA-Z_][a-zA-Z0-9_]*` — no hyphens allowed. This is stricter than zhi's path regex. The workspace template should use underscores in group names and document this constraint.

**Behavioral inventory parameters:** Beyond `ansible_host`/`ansible_port`/`ansible_user`, Ansible recognizes `ansible_connection` (ssh/local/winrm/docker), `ansible_become` (privilege escalation), `ansible_python_interpreter`, `ansible_shell_type`, and others. The template should support these as optional host vars.

### Tree Convention

```
hosts/
  webserver_1/
    ansible_host: "10.0.1.10"
    ansible_user: "deploy"
    ansible_port: "22"
    vars/
      nginx_workers: "4"
groups/
  webservers/
    members: "webserver_1"
    vars/
      http_port: "8080"
ansible/
  config/
    remote_tmp: "/tmp/.ansible/tmp"
    timeout: "30"
```

### Component Mapping

```yaml
components:
  - name: webservers
    paths: ["groups/webservers", "hosts/webserver_1"]
  - name: databases
    paths: ["groups/databases", "hosts/dbserver_1"]
```

### Apply Targets

```yaml
apply:
  targets:
    default:
      command: "ansible-playbook -i inventory/hosts.yml site.yml"
      pre-export: true
    check:
      command: "ansible-playbook -i inventory/hosts.yml site.yml --check --diff"
      pre-export: true
    deploy-web:
      command: "ansible-playbook -i inventory/hosts.yml site.yml --limit webservers"
      pre-export: true
```

### Acceptance Criteria

- [ ] `zhi export` produces valid Ansible inventory YAML
- [ ] `zhi export` produces correct `group_vars/` and `host_vars/` files
- [ ] Disabling a component removes its hosts/groups from exported files
- [ ] Template can be published as OCI artifact via `zhi workspace publish`

---

## Phase 2: Multi-file Export (`iterate`)

**Goal:** Add `iterate` and `output-pattern` fields to the export system so one template can produce multiple output files.

**Scope:** Core changes to `internal/core/` — export system and workspace config.

### Design

When `iterate` is set on an `ExportTemplate`:

1. **Enumerate children:** Scan all tree paths starting with `iterate + "/"`, extract unique first path segments after the prefix. Sort alphabetically.
2. **Materialize child trees:** For each child, materialize a new `config.Tree` containing only that child's paths (with the prefix stripped). This avoids the O(N*M) cost of `prefixTree` wrappers that re-scan the full tree on every `List()` call.
3. **Expand to configs:** For each child, create an `ExportRunConfig` with:
   - Template data: `IterateData{Key: childName, Value: materializedTreeData}` where `Value` is a `TreeData` with relative paths
   - Output path: resolve `output-pattern` as a Go template with `{.Key}` available
4. **Feed to ExpandTemplates:** The expanded configs join the regular configs, getting rollback snapshot coverage for free.

### Research Insights

**Performance (from performance review):**
- `prefixTree.List()` re-scans all M paths on every call. With N children, template methods like `.All()` and `.Nested()` each trigger another full scan, creating O(N*M) total work.
- Materializing child trees into concrete `config.Tree` instances reduces this to O(M) for the initial scan + O(M_child) per child for subsequent operations. At 1,000 hosts with 30,000 paths, this is ~15ms vs ~200ms.

**Security (from security review):**
- `.Key` values are guaranteed safe by the path segment regex (`^[a-z](?:[a-z0-9._-]*[a-z0-9])?$`) — `..` cannot appear as a Key.
- Add `containsPathTraversal` check on raw `output-pattern` string during `ValidateWorkspace` (fail-early, before template expansion).
- After template expansion, `writeExportFile` already rejects `..` (belt-and-suspenders).

**Template contract (from architecture + pattern reviews):**
- Regular templates receive `*TreeData` as dot. Iterate templates receive `*IterateData` — this is a deliberate deviation. The alternative (adding a `Name` field to `TreeData`) would pollute the type for all templates. `IterateData` is explicit about what iterate templates receive.
- Change `renderTemplateFile` data parameter from `*TreeData` to `any`. This is backward-compatible — existing templates still receive `*TreeData`. Adding a parallel `renderIterateTemplateFile` would duplicate template parsing, execution, and security logic.

### Files to Modify

**`internal/core/workspace.go`**

- [x] Add `Iterate` and `OutputPattern` fields to `ExportTemplate` struct (workspace.go:28-34)
- [x] Add validation in `ValidateWorkspace` (workspace.go:219+):
  - Reject if `iterate` set without `output-pattern`
  - Reject if both `output` and `output-pattern` set
  - Parse-check `output-pattern` as Go template (syntax only, no execution)
  - Apply `containsPathTraversal` check on raw `output-pattern` string

```go
type ExportTemplate struct {
	Name          string `yaml:"name" json:"name"`
	Template      string `yaml:"template,omitempty" json:"template,omitempty"`
	Format        string `yaml:"format,omitempty" json:"format,omitempty"`
	Output        string `yaml:"output,omitempty" json:"output,omitempty"`
	Prefix        string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Iterate       string `yaml:"iterate,omitempty" json:"iterate,omitempty"`
	OutputPattern string `yaml:"output-pattern,omitempty" json:"output-pattern,omitempty"`
}
```

**`internal/core/export_data.go`**

- [x] Add `IterateData` struct:

```go
// IterateData is the template data passed when rendering an iterate export.
// .Key is the child name, .Value is a TreeData scoped to the child subtree.
type IterateData struct {
	Key   string
	Value *TreeData
}
```

- [x] Add `directChildren(prefix string) []string` as a **private** helper function (not a TreeData method — only used internally by `ExportIterate`, not needed in templates):

```go
func directChildren(tree config.TreeReader, prefix string) []string {
	p := prefix
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	seen := make(map[string]struct{})
	for _, path := range tree.List() {
		if !strings.HasPrefix(path, p) {
			continue
		}
		rest := strings.TrimPrefix(path, p)
		seg, _, _ := strings.Cut(rest, "/")
		if seg != "" {
			seen[seg] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}
```

- [x] Add `materializeChildTree` helper that creates a concrete `config.Tree` with prefix-stripped paths (avoids O(N*M) re-scanning):

```go
func materializeChildTree(reader config.TreeReader, prefix string, cm *ComponentManager) *TreeData {
	p := prefix
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	tree := config.NewTree()
	for _, path := range reader.List() {
		if strings.HasPrefix(path, p) {
			v, ok := reader.Get(path)
			if ok {
				tree.Set(strings.TrimPrefix(path, p), &v)
			}
		}
	}
	return NewTreeData(tree, cm)
}
```

**`internal/core/export.go`**

- [x] Extend `ExpandTemplates` (from Phase 0 refactoring) with iterate support:
  - For non-iterate templates: convert to `ExportRunConfig` as before
  - For iterate templates: call `directChildren`, materialize per-child tree, resolve output path, create one `ExportRunConfig` per child
- [x] Change `renderTemplateFile` data parameter from `*TreeData` to `any` (backward-compatible)

**`pkg/zhiplugin/ui/ui.go`**

- [x] Add `Iterate` and `OutputPattern` fields to the UI-facing `ExportTemplate` struct (mirrors core struct)

**`internal/ui/adapter.go`**

- [x] Update `ControllerAdapter.ExportTemplates` to propagate `Iterate` and `OutputPattern` fields
- [ ] TUI export view: display iterate templates with a note (e.g., "group-vars (iterates over groups/)")

**`internal/ui/driver.go`**

- [x] `UIController.ExportAll` already uses shared `ExpandTemplates` from Phase 0 — iterate support comes for free

### Edge Cases

- **Empty prefix / no children:** Empty loop produces zero files — natural behavior, no special handling needed
- **Component filtering:** Iterate runs after component filtering (since `PrepareTreeData` filters first), so disabled components' children are naturally excluded
- **iterate + format:** Allow combining `iterate` with `format` (built-in format), not only with `template`

### Tests

- [x] `TestDirectChildren` — unit test for the helper function
- [x] `TestExpandIterateTemplates` — verify expansion produces correct configs; include subtest for zero children
- [x] `TestIterateExport` — end-to-end: tree with groups → output files per child
- [x] `TestIterateWithComponentFiltering` — disabled component's children excluded
- [x] `TestIterateOutputPatternTraversal` — output-pattern producing `../` rejected
- [x] `TestIterateWithFormat` — iterate + built-in format (yaml/json)
- [x] `TestValidateWorkspaceIterate` — mutual exclusivity validation for iterate/output-pattern/output

Note: do NOT use `t.Parallel()` in `internal/core/` tests — existing tests in that package don't use it.

### Acceptance Criteria

- [x] `zhi export` with iterate config produces one file per direct child of the prefix
- [x] Templates receive `{.Key, .Value}` with relative paths in `.Value`
- [x] Rollback snapshot covers all iterated output files (automatic via `ExportAll`)
- [x] Workspace validation rejects invalid iterate/output-pattern combinations
- [x] `--dry-run` and `--diff` work correctly with iterate exports
- [x] Zero children under prefix produces zero files without error

---

## Phase 3: Ansible Config Plugin

**Goal:** A config plugin (`zhi-config-ansible`) that reads existing Ansible YAML inventory and variable files, exposing them as a zhi tree with write-back support.

**Scope:** New external plugin binary in `examples/zhi-config-ansible/`.

### Design

The plugin reads an Ansible project's YAML inventory and `group_vars/`/`host_vars/` directories, mapping them to zhi's flat path model. Each value carries an `ansible.source_file` metadata key tracking its origin file for write-back.

**Tree mapping:**

| Ansible source | zhi path | Example |
|---|---|---|
| Inventory host entry | `hosts/<hostname>/ansible_host` | `hosts/web1/ansible_host` = `"10.0.1.10"` |
| Inventory group membership | `groups/<group>/members` | `groups/webservers/members` = `"web1,web2"` |
| `host_vars/<host>/<file>.yml` key | `hosts/<host>/vars/<key>` | `hosts/web1/vars/nginx_workers` = `"4"` |
| `group_vars/<group>/<file>.yml` key | `groups/<group>/vars/<key>` | `groups/webservers/vars/http_port` = `"8080"` |

**Plugin options:**

```yaml
config:
  provider: ansible
  options:
    inventory: ./inventory/hosts.yml
    group_vars_dir: ./group_vars
    host_vars_dir: ./host_vars
```

### Research Insights

**Config plugin skill (14 findings applied):**
- Numeric values arrive as `float64` after gRPC JSON round-trip — `ansible_port` validation must type-assert `float64`, not `int`
- Never populate `Value.Validators` field — closures don't cross gRPC wire
- `Set` must call `config.ValidatePath(path)` to reject invalid path segments
- `Get` must return a copy (`return *v, true, nil`), not a pointer to internal state
- `Validate` must read from the `tree` parameter (engine's snapshot), not internal cache
- Return `nil, nil` from `Validate` when path not found in tree (early return)
- All `Metadata` values must be JSON-primitive types (string, float64, bool, etc.)

**Label registration doesn't work across process boundaries:** The plugin runs as a separate gRPC process. Registering a label in the plugin's `init()` has no effect on the host's `DefaultRegistry`. Just use `"ansible.source_file"` as a plain metadata key — unknown labels are allowed (the registry returns no error for them).

**Security (from security review):**
- **Confine writes to project root.** Resolve `ansible.source_file` relative to the Ansible project root. Reject absolute paths. Use `filepath.Rel` after `filepath.EvalSymlinks` to verify containment. Apply `containsPathTraversal` check.
- **File size limit.** Check file size with `os.Stat` before reading (reject files > 10 MB to prevent DoS from malformed YAML).

**Simplicity considerations:**
- V1 only supports flat key-value YAML var files. If a var file contains nested YAML, flatten one level deep only. Do not attempt general recursive flattening.
- Start with 3 core validation rules: host-in-group existence, port-is-numeric, and group-name-format. Add more when users ask.
- Drop comment-preserving YAML round-trip — rabbit hole with no demonstrated need.

### Files to Create

**`examples/zhi-config-ansible/main.go`**

- [x] Implement `ansiblePlugin` struct with `sync.RWMutex`, inventory path, group/host vars dirs, in-memory `map[string]*config.Value` cache, and dirty file tracker
- [x] Set up `hclog` logger with `ZHI_LOG_LEVEL` env var support (matching pokedex pattern)
- [x] Plugin receives options via workspace config `options` map (inventory path, var dirs)
- [x] `List(ctx)` — return keys from in-memory cache under `RLock`
- [x] `Get(ctx, path)` — return `*v` (copy) with `ansible.source_file` in Metadata, under `RLock`
- [x] `Set(ctx, path, value)`:
  - Call `config.ValidatePath(path)` first
  - Update in-memory cache under `Lock`
  - Determine target file from existing `ansible.source_file` metadata or derive from path convention
  - Validate target file is within project root (path confinement)
  - Flush the affected source file immediately (synchronous write-back, atomic temp+rename)
- [x] `Validate(ctx, path, tree)`:
  - Early return `nil, nil` if path not found in `tree`
  - Use `tree.Get`/`tree.List` for cross-value checks (not internal cache)
  - Rule 1: Host references in groups must exist (`groups/<g>/members` → each member must have `hosts/<member>/...`)
  - Rule 2: `ansible_port` should be numeric 1-65535 (type-assert `float64`, check no fractional part)
  - Rule 3: Group names must match `[a-zA-Z_][a-zA-Z0-9_]*` (Ansible restriction — no hyphens)
- [x] `loadInventory()` — parse YAML inventory file (check size < 10 MB first), build hosts/groups tree. Inventory structure: root is `all` with `hosts`, `children`, `vars` sub-keys per group
- [x] `loadVarFiles(dir, prefix)` — scan `group_vars/` or `host_vars/` directories, flatten one level of nesting only
- [x] `main()` — `goplugin.Serve` with `zhiplugin.Handshake`, `config.GRPCPlugin{Impl: plugin}`, `hclog` logger

**`examples/zhi-config-ansible/main_test.go`**

- [x] `dispense` helper using `goplugin.TestPluginGRPCConn` pattern
- [x] Test `List`/`Get` with fixture YAML inventory and var files
- [x] Test round-trip: `Get` → `Set` (modified value) → `Get` verifies value persisted
- [x] Test validation: missing host in group membership → Blocking result
- [x] Test validation: invalid `ansible_port` (non-numeric, out of range) → Warning result
- [x] Build `config.Tree` from plugin's `List`/`Get` output for `Validate` calls (standard pattern)
- [x] Test path confinement: `Set` with `ansible.source_file` pointing outside project root → error

**`examples/zhi-config-ansible/go.mod`**

- [x] Part of main module (no separate go.mod needed, gopkg.in/yaml.v3 already in main go.mod)

**`examples/zhi-config-ansible/zhi-plugin.yaml`**

- [x] Plugin manifest with `type: config`, `name: ansible`

**`examples/zhi-config-ansible/testdata/`**

- [x] Fixture: `inventory/hosts.yml` with sample groups and hosts
- [x] Fixture: `group_vars/webservers.yml` with sample vars
- [x] Fixture: `host_vars/web1/main.yml` with sample vars

### Write-back Strategy

1. On construction: Load all files into memory, cache values with `ansible.source_file` metadata
2. On `Set`: Validate path, update cache, flush the affected source file immediately (no lazy buffering — avoids data loss window)
3. For new paths with no existing source file: derive target file from path convention:
   - `hosts/<host>/vars/<key>` → `host_vars/<host>/main.yml`
   - `groups/<group>/vars/<key>` → `group_vars/<group>/main.yml`
4. Path confinement: all write targets must resolve within the configured project root

### Acceptance Criteria

- [ ] Plugin reads YAML inventory and var files into a zhi tree
- [ ] `zhi tree` (via TUI or CLI) shows Ansible hosts, groups, and variables
- [ ] Editing a value via TUI writes it back to the correct Ansible file
- [ ] Creating a new host/group path creates the appropriate var file
- [ ] Write-back is confined to project root — paths outside are rejected
- [ ] Validation catches missing hosts in groups and invalid port numbers
- [ ] Plugin publishes as OCI artifact via `zhi plugin publish`

---

## Phase 4: Pre-checks

**Goal:** Add `pre-check` field to apply targets — commands that must succeed before the main command runs.

**Scope:** Small core changes to `internal/core/` — apply system and workspace config.

### Design

- `pre-check` is a list of shell commands on `ApplyTargetConfig`
- Each runs sequentially, inheriting `workdir`, `env` from the parent target
- Output is streamed via the existing `ApplyOutput` channel
- Any non-zero exit aborts the apply with an error identifying which pre-check failed (command index, text, exit code)
- Execution order: pre-export → pre-check → main command
- Timeout: the target's timeout applies to the entire sequence (pre-checks + main command) via a shared context

### Research Insights

**Security (from security review):**
- Pre-check commands run via `sh -c` — same trust model as existing apply commands. User-controlled `zhi.yaml` is the command source, which is inherently trusted.
- Document clearly that env var values are not shell-escaped when referenced in commands.

**Performance (from performance review):**
- Keep sequential execution. Parallel would interleave output, cause SSH connection contention, and break implicit ordering dependencies between pre-checks. The time savings are marginal compared to a multi-minute playbook run.

### Files to Modify

**`internal/core/workspace.go`**

- [x] Add `PreCheck []string` field to `ApplyTargetConfig` (workspace.go:42-48)

```go
type ApplyTargetConfig struct {
	Command   string            `yaml:"command,omitempty" json:"command,omitempty"`
	Workdir   string            `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	PreExport bool              `yaml:"pre-export,omitempty" json:"pre-export,omitempty"`
	PreCheck  []string          `yaml:"pre-check,omitempty" json:"pre-check,omitempty"`
	Env       map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Timeout   int               `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}
```

**`internal/core/apply.go`**

- [x] Add `RunPreChecks` function that iterates over `PreCheck` commands:
  - Each command runs via `sh -c` with same workdir/env as main command (reuse `buildApplyEnv`)
  - Output streams to the same `ApplyOutput` channel
  - On non-zero exit: return error with command index, command text, and exit code
  - On success: continue to next pre-check

**`internal/cli/apply.go`**

- [x] Insert pre-check execution between pre-export and main command (around line 121)
- [x] `--dry-run`: print pre-check commands along with main command
- [x] Stream pre-check output to terminal

**`internal/ui/adapter.go` or `internal/ui/driver.go`**

- [x] Add pre-check orchestration to UIController (not just CLI) so TUI apply also runs pre-checks
- [x] Display which pre-check is running (e.g., "[pre-check 1/2] ansible all -m ping")

### Tests

- [x] `TestPreCheckSuccess` — all pre-checks pass, main command runs
- [x] `TestPreCheckFailure` — second pre-check fails, main command does not run, error identifies which command
- [x] `TestPreCheckEmpty` — no pre-checks configured, main command runs normally
- [x] `TestPreCheckDryRun` — dry-run prints pre-check commands
- [x] `TestPreCheckOutputStreamed` — pre-check stdout/stderr appears in output channel

Note: workdir/env inheritance is structural (reuses `buildApplyEnv`) — no separate tests needed.

### Acceptance Criteria

- [ ] Pre-checks run after pre-export, before main command
- [ ] Failed pre-check aborts apply with clear error message (command index + text + exit code)
- [ ] Pre-check output is streamed to user in real-time
- [ ] Pre-checks inherit workdir and env from parent target
- [ ] `--dry-run` shows pre-check commands
- [ ] Empty pre-check list is a no-op (backward compatible)
- [ ] Target timeout covers entire sequence (pre-checks + main command)

---

## References

### Internal

- Design document: `docs/design/ansible-integration.md`
- Export system: `internal/core/export.go:51` (`Export`), `internal/core/export.go:105` (`ExportAll`)
- Export data: `internal/core/export_data.go:13` (`TreeData`)
- Workspace config: `internal/core/workspace.go:28` (`ExportTemplate`), `internal/core/workspace.go:42` (`ApplyTargetConfig`)
- Apply system: `internal/core/apply.go:56` (`Apply`)
- Config plugin interface: `pkg/zhiplugin/config/config.go` (line 14 regex, `Value` struct)
- Example config plugin: `examples/zhi-config-pokedex/main.go`
- CLI export: `internal/cli/export.go` (ExportTemplate-to-ExportRunConfig at line 115)
- CLI apply: `internal/cli/apply.go`
- UI controller: `internal/ui/driver.go:253` (ExportAll — duplicated conversion logic)
- UI adapter: `internal/ui/adapter.go:62` (ExportTemplate field propagation)
- UI plugin ExportTemplate: `pkg/zhiplugin/ui/ui.go:41`
- TUI export view: `internal/ui/tui/export_view.go`
- TUI apply view: `internal/ui/tui/apply_view.go`
