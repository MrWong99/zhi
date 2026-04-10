# Lesson 4: Workspaces and the Marketplace

This lesson covers installing pre-built workspaces from OCI registries (which automatically install their plugin dependencies), browsing the plugin marketplace through the web UI, and managing plugins.

## Prerequisites

- Completed [Lesson 1](lesson-01-getting-started.md) through [Lesson 3](lesson-03-components.md)
- zhi installed

## Workspaces from OCI Registries

A zhi workspace is a self-contained configuration project: `zhi.yaml`, config files, templates, and apply scripts. Workspaces can be packaged as OCI artifacts and shared via container registries -- just like Docker images but for configuration projects.

When you install a workspace, zhi:

1. Pulls the workspace artifact from the registry
2. Extracts its files into the target directory
3. Installs any declared plugin dependencies automatically

### Step 1: Install the zhi Deployment Workspace

The zhi project publishes pre-built workspaces for common deployment scenarios. Install the zhi deployment workspace (which manages a zhi-mirror and zhi-marketplace deployment):

```sh
zhi workspace install ghcr.io/mrwong99/zhi/zhi-workspace-zhi:v1.10.2 ./my-zhi-deployment --skip-tools-check
```

You should see output like:

```
Pulling workspace ghcr.io/mrwong99/zhi/zhi-workspace-zhi:v1.10.2...

Dependencies:
  + oci://ghcr.io/mrwong99/zhi/zhi-store-vault:v1.10.2

Workspace installed to ./my-zhi-deployment
```

Notice that the `zhi-store-vault` plugin was automatically installed because the workspace declares it as a dependency in its `zhi-workspace.yaml` manifest:

```yaml
name: zhi
version: 1.10.2
description: This zhi deployment can be used to deploy a zhi-mirror and zhi-marketplace.
dependencies:
  - ref: oci://ghcr.io/mrwong99/zhi/zhi-store-vault:v1.10.2
    type: store
    optional: false
```

You can verify the plugin was installed:

```sh
zhi plugin list
```

The `vault` store plugin should appear in the list.

### Available Official Workspaces

| Workspace | OCI Reference | Description |
|-----------|---------------|-------------|
| vault | `ghcr.io/mrwong99/zhi/zhi-workspace-vault` | Deploy a HashiCorp Vault instance |
| zhi | `ghcr.io/mrwong99/zhi/zhi-workspace-zhi` | Deploy zhi-mirror and zhi-marketplace |

### Useful Flags

| Flag | Description |
|------|-------------|
| `--skip-plugins` | Do not install plugin dependencies |
| `--skip-tools-check` | Do not check for required external tools |
| `--dry-run` | Show what would be installed without doing it |

## Browsing the Marketplace with the Web UI

The web UI includes a built-in marketplace browser where you can search, explore, and install plugins visually.

### Step 2: Launch the Web UI

Navigate to a workspace and launch the web UI:

```sh
cd ./my-zhi-deployment  # or any workspace directory
zhi edit --ui webui
```

This starts a local web server and opens your browser. The default address is `http://127.0.0.1:8080`.

### Step 3: Navigate to the Marketplace

In the web UI, use the navigation bar to reach the **Marketplace** page. You can also use the keyboard shortcut `g m` (press `g` then `m`).

The marketplace page lets you:

- **Search** by name or keyword
- **Filter** by plugin type (config, transform, store, ui)
- **Sort** by relevance, downloads, or rating
- **View details** -- publisher, version, rating, download count, signing status
- **Install** plugins directly from the browser

### Step 4: View Installed Plugins

Navigate to the **Plugins** page (keyboard shortcut `g p`) to see all installed plugins. From here you can:

- Check version and verification status
- Update individual plugins or all at once
- Uninstall external plugins

## Managing Plugins from the CLI

You can also manage plugins entirely from the command line.

### Installing Plugins

```sh
# Install from an OCI reference
zhi plugin install oci://ghcr.io/mrwong99/zhi/zhi-store-memory:v1.10.2

# Install from a marketplace short name (if a marketplace is configured)
zhi plugin install ansible-config
```

### Searching

```sh
zhi plugin search ansible
zhi plugin search --type store
```

### Plugin Information

```sh
zhi plugin info vault
```

### Updating Plugins

```sh
# Check for available updates
zhi plugin update --check

# Update a specific plugin
zhi plugin update vault

# Update all plugins
zhi plugin update --all
```

### Uninstalling

```sh
zhi plugin uninstall memory
```

## Publishing Your Own Workspace

Once you have built a workspace you want to share, you can publish it as an OCI artifact:

```sh
zhi workspace publish --registry ghcr.io/myorg
```

Include a `zhi-workspace.yaml` manifest in your workspace directory to declare metadata, dependencies, and required tools:

```yaml
name: my-workspace
version: 1.0.0
description: My custom deployment workspace
author: myorg
license: MIT
dependencies:
  - ref: oci://ghcr.io/mrwong99/zhi/zhi-store-json:v1.10.2
    type: store
tools:
  - name: docker
    version: "29.0.0"
keywords:
  - deployment
  - docker
```

## Summary

In this lesson you learned how to:

- Install pre-built workspaces from OCI registries with `zhi workspace install`
- Understand that workspace dependencies (plugins) are installed automatically
- Browse and install plugins using the web UI marketplace
- Manage plugins from the CLI (install, search, update, uninstall)
- Publish your own workspaces

## Further Reading

- [Sharing and Registries](../user-guide/sharing-and-registries.md) -- full guide on installing, publishing, and securing plugins and workspaces
- [Plugin Discovery](../user-guide/plugin-discovery.md) -- filesystem-based plugin discovery and naming conventions
- [Web UI](../user-guide/web-ui.md) -- web UI configuration, features, and keyboard shortcuts
- [CLI Reference](../user-guide/cli-reference.md#zhi-workspace) -- complete `zhi workspace` command reference
- [CLI Reference](../user-guide/cli-reference.md#zhi-plugin) -- complete `zhi plugin` command reference
- [Enterprise Mirror](../user-guide/enterprise-mirror.md) -- local OCI mirror for air-gapped environments
