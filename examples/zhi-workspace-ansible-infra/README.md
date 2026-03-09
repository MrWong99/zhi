# zhi-workspace-ansible-infra

A zhi workspace template for managing Ansible infrastructure. It provides
ready-made export templates that generate Ansible inventory files, variable
files, and configuration from a zhi configuration tree.

## Quick Start

```bash
zhi workspace new --from oci://ghcr.io/mrwong99/zhi/zhi-workspace-ansible-infra:latest my-infra
cd my-infra
# Edit config.yaml to define your hosts and groups
zhi export          # Generate all Ansible files
zhi apply           # Run ansible-playbook
```

## Tree Convention

The configuration tree follows this structure:

```
hosts/
  <hostname>/
    ansible_host: "<ip-or-hostname>"
    ansible_user: "<ssh-user>"
    ansible_port: "<port>"
    vars/
      <key>: "<value>"

groups/
  <groupname>/
    members: "<host1>,<host2>"
    vars/
      <key>: "<value>"
    vault/
      <key>: "<secret-value>"

ansible/
  config/
    remote_tmp: "/tmp/.ansible/tmp"
    timeout: "30"
```

- **hosts/** -- One entry per managed host with Ansible connection parameters
  and host-specific variables under `vars/`.
- **groups/** -- One entry per Ansible group. The `members` field is a
  comma-separated list of hostnames. Group variables go under `vars/`.
  Secrets go under `vault/` (exported separately).
- **ansible/config/** -- Maps to `ansible.cfg` settings.

## Export Templates

| Template | Output | Description |
|----------|--------|-------------|
| `inventory.yml.tmpl` | `ansible/inventory/hosts.yml` | YAML-format inventory |
| `inventory.ini.tmpl` | `ansible/inventory/hosts.ini` | INI-format inventory |
| `ansible.cfg.tmpl` | `ansible/ansible.cfg` | Ansible configuration |
| `group-vars.yml.tmpl` | `ansible/group_vars/<group>.yml` | Per-group variables (one file per group) |
| `host-vars.yml.tmpl` | `ansible/host_vars/<host>.yml` | Per-host variables (one file per host) |
| `vault-vars.yml.tmpl` | `ansible/group_vars/<group>/vault.yml` | Per-group secrets from zhi store |

The `group-vars`, `host-vars`, and `vault-vars` templates use the `iterate`
feature to produce one output file per tree child.

## Components

Components map to logical groups of infrastructure. Disabling a component
removes its hosts and groups from all generated files.

```yaml
components:
  - name: webservers
    paths: ["groups/webservers/", "hosts/webserver-1/"]
  - name: databases
    paths: ["groups/databases/", "hosts/dbserver-1/"]
```

Toggle components with `zhi component enable/disable <name>`.

## Apply Targets

| Target | Command | Description |
|--------|---------|-------------|
| `default` | `ansible-playbook ... site.yml` | Run the full playbook |
| `check` | `ansible-playbook ... --check --diff` | Dry-run with diff output |
| `deploy-web` | `ansible-playbook ... --limit webservers` | Deploy only to webservers |
| `destroy` | `ansible-playbook ... teardown.yml` | Tear down infrastructure |

Run a target: `zhi apply` (default) or `zhi apply --target check`.

## Adding Hosts and Groups

1. Add the host under `hosts/` in `config.yaml`:
   ```yaml
   hosts:
     new-server:
       ansible_host: "10.0.2.10"
       ansible_user: deploy
       ansible_port: "22"
       vars:
         some_setting: "value"
   ```

2. Add or update a group under `groups/`:
   ```yaml
   groups:
     webservers:
       members: "webserver-1,new-server"
       vars:
         http_port: "8080"
   ```

3. Optionally create a component for the new group in `zhi.yaml`.

4. Run `zhi export` to regenerate all Ansible files.

## Requirements

- [Ansible](https://docs.ansible.com/) >= 2.9 (`ansible-playbook` on PATH)
- [zhi](https://github.com/MrWong99/zhi) with iterate export support
