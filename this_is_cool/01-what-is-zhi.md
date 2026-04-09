# Lesson 1: What is zhi?

**zhi** is a security-first platform for configuration management and provisioning.
Think of it as a framework where you define *what* your configuration looks like,
*how* it gets transformed, *where* it's stored, and *how* you interact with it --
all through a plugin system.

## The Big Picture

```text {"excludeFromRunAll": true}
┌──────────────┐     ┌───────────────┐     ┌──────────────┐
│   Config     │────▶│   Transform   │────▶│    Store     │
│   Plugin     │     │   Plugin      │     │    Plugin    │
│              │     │               │     │              │
│ Defines the  │     │ Mutates or    │     │ Persists the │
│ structure &  │     │ validates the │     │ config tree  │
│ defaults     │     │ tree          │     │ (Vault, JSON,│
│              │     │               │     │  memory...)  │
└──────────────┘     └───────────────┘     └──────────────┘
        │                                         │
        └──────────────┐  ┌───────────────────────┘
                       ▼  ▼
                 ┌─────────────┐
                 │  UI Plugin  │
                 │             │
                 │ TUI, Web,   │
                 │ MCP/AI,     │
                 │ HTTP API    │
                 └─────────────┘
```

Four plugin types, one composable system. Let's install it and see for ourselves.

---

## Step 1: Install zhi

We'll download the latest release from GitHub. The script auto-detects your OS and architecture.

```sh {"name": "install-zhi", "interactive": true}
# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

# Get the latest release tag
LATEST=$(curl -s https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')
VERSION="${LATEST#v}"
echo "Latest version: $LATEST ($OS/$ARCH)"

# Download and extract
TARBALL="zhi_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -sL "https://github.com/MrWong99/zhi/releases/download/${LATEST}/${TARBALL}" -o "/tmp/${TARBALL}"
tar -xzf "/tmp/${TARBALL}" -C /tmp zhi

# Move to a directory in your PATH
sudo mv /tmp/zhi /usr/local/bin/zhi
rm "/tmp/${TARBALL}"

echo "Installed zhi ${VERSION} to /usr/local/bin/zhi"
```

### Verify the installation

```sh {"name": "verify-zhi"}
zhi version
```

> **Try it yourself:** If you prefer a different install location, adjust the `mv` target above.
> You can also download directly from [GitHub Releases](https://github.com/MrWong99/zhi/releases).

---

## Step 2: Explore the CLI

zhi's CLI is organized into subcommands. Let's see what's available.

```sh {"name": "zhi-help"}
zhi --help
```

The key commands you'll use throughout this tutorial:

| Command | Purpose |
|---------|---------|
| `zhi init` | Create a new workspace |
| `zhi list paths` | Browse the configuration tree |
| `zhi get <path>` | Read a specific value |
| `zhi set <path> <value>` | Update a value |
| `zhi validate` | Check all validation rules |
| `zhi export` | Render templates to files |
| `zhi apply` | Run provisioning commands |
| `zhi edit` | Open the interactive editor (TUI/Web/MCP) |

### Try a few commands

```sh {"name": "zhi-list-help", "interactive": true}
# Explore subcommand help
zhi list paths --help
```

```sh {"name": "zhi-plugin-help", "interactive": true}
# Plugin management commands
zhi plugin --help
```

---

## Step 3: Understand the Workspace

Every zhi project starts with a **workspace** -- a `zhi.yaml` file that declares:

```yaml {"excludeFromRunAll": true}
# Example zhi.yaml structure (don't run this, just read!)
version: "1"

config:
  provider: structuredfile        # Where config definitions live
  options:
    directory: ./config           # YAML files defining the tree

transform:                        # Optional: mutate/validate the tree
  - provider: my-transform

store:
  provider: vault                 # Where values are persisted
  options:
    addr: http://127.0.0.1:8200

ui:
  provider: webui                 # How you interact with it
  options:
    addr: 127.0.0.1:8080

components:                       # Logical groupings of config paths
  - name: database
    paths: ["database/"]

export:                           # Files to generate from config
  templates:
    - name: docker-compose
      template: ./templates/docker-compose.yml.tmpl
      output: ./docker-compose.yml

apply:
  command: docker compose up -d   # Provisioning command
```

The magic: everything is **pluggable**. Swap `vault` for `json` storage, replace `webui`
with `tui` or MCP -- the config tree stays the same.

---

## Check your work

```sh {"name": "check-01"}
if command -v zhi &>/dev/null; then
  echo "✓ zhi is installed: $(zhi version 2>&1 | head -1)"
else
  echo "✗ zhi not found in PATH. Check the installation step above."
  exit 1
fi
```

---

**Next up:** [Lesson 2 - Your First Workspace](02-your-first-workspace.md) -- we'll create
a workspace from scratch and explore the config tree hands-on.
