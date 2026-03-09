# Design: Ansible Integration for zhi

## Motivation

zhi already supports triggering external provisioning tools (Docker Compose, kubectl, etc.) via the `apply` system, which shells out to a command after optionally exporting config files. However, Ansible has unique characteristics that make a deeper integration valuable:

1. **Remote execution** -- Ansible manages remote hosts over SSH, not just the local machine.
2. **Inventory management** -- Ansible needs a dynamic or static inventory of hosts/groups.
3. **Variable layering** -- Ansible has its own variable precedence system (group_vars, host_vars, extra-vars) that maps naturally to zhi's tree model.
4. **Idempotent playbooks** -- Ansible playbooks are declarative; zhi's component model can selectively enable/disable roles.

The goal is to make zhi a first-class configuration source for Ansible without reimplementing Ansible itself.

## Current State

Today, Ansible integration is possible but manual:

```yaml
# zhi.yaml
export:
  templates:
    - name: ansible-vars
      template: ./templates/group_vars.yml.tmpl
      output: ./ansible/group_vars/all.yml
    - name: inventory
      template: ./templates/inventory.ini.tmpl
      output: ./ansible/inventory.ini

apply:
  command: "ansible-playbook -i ansible/inventory.ini site.yml"
  pre-export: true
```

This works but requires users to manually author templates for inventory files and variable files, understand Ansible's directory conventions, and glue everything together.

## Proposed Integration Layers

The integration is designed as three independent, incrementally adoptable layers. Each layer builds on zhi's existing extension points rather than adding new plugin types.

---

### Layer 1: Ansible Export Templates (Workspace Template)

**What:** A publishable workspace template (`zhi workspace new --from template/ansible-infra`) providing ready-made export templates for common Ansible artifacts.

**Scope:** Templates only -- no new code in zhi core.

**Provided templates:**

| Template | Output | Description |
|----------|--------|-------------|
| `inventory.ini.tmpl` | `inventory/hosts` | INI-format static inventory generated from `hosts/` tree paths |
| `inventory.yml.tmpl` | `inventory/hosts.yml` | YAML-format inventory (alternative) |
| `group-vars.yml.tmpl` | `group_vars/<group>.yml` | Per-group variable files from `groups/<name>/` tree prefix |
| `host-vars.yml.tmpl` | `host_vars/<host>.yml` | Per-host variable files from `hosts/<name>/vars/` tree prefix |
| `vault-vars.yml.tmpl` | `group_vars/<group>/vault.yml` | Ansible Vault-encrypted secrets (leveraging zhi store encryption) |
| `ansible.cfg.tmpl` | `ansible.cfg` | Ansible configuration from `ansible/config/` tree prefix |

**Tree convention:**

```
hosts/
  webserver-1/
    ansible-host: "10.0.1.10"
    ansible-user: "deploy"
    ansible-port: "22"
    vars/
      nginx-workers: "4"
  dbserver-1/
    ansible-host: "10.0.1.20"
    ansible-user: "deploy"
    vars/
      pg-max-connections: "200"
groups/
  webservers/
    members: "webserver-1"
    vars/
      http-port: "8080"
  databases/
    members: "dbserver-1"
    vars/
      backup-schedule: "0 2 * * *"
ansible/
  config/
    remote-tmp: "/tmp/.ansible/tmp"
    timeout: "30"
```

**Component mapping:**

```yaml
components:
  - name: webservers
    paths: ["groups/webservers", "hosts/webserver-1"]
  - name: databases
    paths: ["groups/databases", "hosts/dbserver-1"]
  - name: monitoring
    paths: ["groups/monitoring"]
```

Disabling the `monitoring` component removes those hosts from the generated inventory and variable files.

**Apply targets:**

```yaml
apply:
  default:
    command: "ansible-playbook -i inventory/hosts.yml site.yml"
    pre-export: true
  check:
    command: "ansible-playbook -i inventory/hosts.yml site.yml --check --diff"
    pre-export: true
  deploy-web:
    command: "ansible-playbook -i inventory/hosts.yml site.yml --limit webservers"
    pre-export: true
  destroy:
    command: "ansible-playbook -i inventory/hosts.yml teardown.yml"
    pre-export: true
```

**Deliverable:** A workspace template published as an OCI artifact to the marketplace.

---

### Layer 2: Ansible Dynamic Inventory Plugin (Config Plugin)

**What:** A zhi **config plugin** (`zhi-config-ansible`) that reads an existing Ansible project's inventory and variable files and exposes them as a zhi configuration tree.

**Why config plugin?** The config plugin interface (`List`, `Get`, `Set`, `Validate`) maps directly to reading/writing Ansible inventory and variables. This is not a new plugin type -- it's a standard config provider.

**Behavior:**

- `List()` -- Scans Ansible inventory (INI/YAML/dynamic script), `group_vars/`, `host_vars/`, and returns all discovered paths.
- `Get(path)` -- Returns the value at a tree path, e.g. `Get("hosts/web-1/ansible-host")` returns `"10.0.1.10"`.
- `Set(path, value)` -- Writes back to the appropriate Ansible file (e.g., updating a host var writes to `host_vars/<host>/main.yml`).
- `Validate(path, tree)` -- Cross-validates: host references in groups exist, required variables (like `ansible_host`) are set, SSH port is numeric, etc.

**Plugin options (via `zhi.yaml`):**

```yaml
config:
  provider: ansible
  options:
    inventory: ./inventory/hosts.yml    # or hosts.ini, or a dynamic inventory script
    group-vars-dir: ./group_vars
    host-vars-dir: ./host_vars
    ansible-cfg: ./ansible.cfg          # optional, for additional settings
```

**Validation rules (built-in):**

- Every host in a group must exist in the inventory
- `ansible_host` should be a valid IP or hostname
- `ansible_port` should be numeric and in valid range
- `ansible_user` should be non-empty
- Warn on hosts that belong to no group
- Warn on unreferenced group variables

**Reverse direction:** This plugin enables managing an *existing* Ansible project through zhi's TUI/UI, with validation and component toggling, without restructuring the project.

**Deliverable:** A Go plugin binary published to the marketplace.

---

### Layer 3: Ansible Apply Enhancements (Core Enhancement)

**What:** Small, targeted additions to zhi's apply system that benefit Ansible users (and other SSH-based tools).

#### 3a. Multi-File Export Support

Currently, each export template produces one file. Ansible projects need multiple files generated from a single tree (one `group_vars/<name>.yml` per group). Add a **glob output** mode to the export system:

```yaml
export:
  templates:
    - name: group-vars
      template: ./templates/group-vars.yml.tmpl
      output-pattern: "./group_vars/{{ .Key }}.yml"
      iterate: groups          # tree prefix to iterate over
```

**Semantics:** When `iterate` is set, the export system calls the template once per direct child of the given prefix, setting `.Key` to the child name and `.Value` to the subtree. The `output-pattern` is itself a Go template that determines the output filename.

This is the single most impactful core change -- it eliminates the need for wrapper scripts that loop over groups/hosts.

#### 3b. Apply Output Parsing (Ansible Recap)

Add optional structured output parsing for apply results. Ansible's `--callback json` produces machine-readable output. A new `parse` field in apply config:

```yaml
apply:
  default:
    command: "ansible-playbook -i inventory/hosts.yml site.yml --callback json"
    pre-export: true
    parse: ansible-json       # built-in parser for Ansible JSON callback output
```

Parsed results surface in the TUI/UI as structured data: per-host ok/changed/failed/skipped counts, task-level details, and failure messages. This is optional and degrades gracefully (raw output shown if parsing fails).

Built-in parsers: `ansible-json`. Extensible via a simple interface for future tools.

#### 3c. SSH Host Key Pre-validation

Before running a playbook, optionally validate SSH connectivity to inventory hosts:

```yaml
apply:
  default:
    command: "ansible-playbook -i inventory/hosts.yml site.yml"
    pre-export: true
    pre-check:
      - "ansible -i inventory/hosts.yml all -m ping --one-line"
```

The `pre-check` field is a list of commands that must succeed (exit 0) before the main `command` runs. This is generic (not Ansible-specific) and useful for any pre-flight validation.

---

## Integration Matrix

| Layer | Requires Core Changes | Plugin Type | Difficulty | Standalone Value |
|-------|----------------------|-------------|------------|------------------|
| Layer 1: Templates | No | Workspace template | Low | High -- immediate usability |
| Layer 2: Config Plugin | No | Config plugin | Medium | High -- bidirectional Ansible management |
| Layer 3a: Multi-file export | Yes (export system) | Core | Medium | High -- unblocks many template patterns |
| Layer 3b: Output parsing | Yes (apply system) | Core | Low | Medium -- better UX |
| Layer 3c: Pre-checks | Yes (apply system) | Core | Low | Medium -- generic pre-flight |

## Recommended Implementation Order

1. **Layer 1** -- Templates. Ship immediately as a marketplace workspace template. Zero risk, high value. This validates the tree convention before building a plugin around it.

2. **Layer 3a** -- Multi-file export. This is the highest-value core change and benefits all users, not just Ansible. Implement `iterate` + `output-pattern` in `internal/core/export.go`.

3. **Layer 2** -- Config plugin. Build after the tree convention is validated. Publish as `zhi-config-ansible` on the marketplace.

4. **Layer 3c** -- Pre-checks. Small, generic addition to the apply config. Useful beyond Ansible.

5. **Layer 3b** -- Output parsing. Nice-to-have. Can be deferred until there's demand.

## Alternatives Considered

### New "provisioner" plugin type

Adding a fifth plugin type (`provisioner`) that natively speaks Ansible's API was considered and rejected. Reasons:

- Ansible is invoked as a CLI tool, not a library. There's no stable Go API.
- A new plugin type adds permanent complexity to the plugin framework (handshake, gRPC proto, registry, discovery, lifecycle).
- The apply system already handles CLI invocation well. The gap is in *file generation* (solved by Layer 3a) and *reading existing projects* (solved by Layer 2).

### Embedding Ansible inventory as a dynamic inventory script

Instead of a config plugin, zhi could act as an [Ansible dynamic inventory script](https://docs.ansible.com/ansible/latest/dev_guide/developing_inventory.html) that Ansible calls to get hosts. This was considered as a complement (not replacement):

```sh
ansible-playbook -i "zhi inventory --format ansible-json" site.yml
```

This is a useful CLI subcommand to add (`zhi inventory`) but is less powerful than the config plugin approach because it's one-directional (read-only, Ansible pulls from zhi) and doesn't enable managing Ansible projects *through* zhi's UI.

**Recommendation:** Add `zhi inventory` as a thin CLI command that calls `zhi export --format ansible-inventory` -- this falls out naturally from Layer 1 templates.

## Non-Goals

- **Reimplementing Ansible** -- zhi will not execute SSH commands, manage playbook logic, or replace `ansible-playbook`.
- **Ansible Vault integration** -- zhi has its own store encryption. Interop with Ansible Vault's encryption format is out of scope (users can use zhi's store to manage secrets and export them as plaintext to Ansible, relying on zhi's access controls instead).
- **AWX/Tower API integration** -- Out of scope for the initial design. Could be a future UI plugin that calls the AWX API.
