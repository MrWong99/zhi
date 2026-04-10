# Lesson 4: zhi Infrastructure

Now that Vault is running, we'll deploy zhi's own infrastructure:

- **zhi-mirror** -- a pull-through cache and policy engine for plugins
- **zhi-marketplace** -- a registry for browsing, searching, and rating plugins

The zhi project publishes **pre-built workspaces** to the GitHub Container Registry.
Instead of configuring everything from scratch, we'll install them with
`zhi workspace install` -- which pulls the workspace files *and* automatically
installs any required plugins.

---

## Step 1: Install the Vault Workspace from OCI

The Vault workspace we used in Lesson 3 is also available as an OCI artifact.
Let's see how `zhi workspace install` works by installing it into a fresh directory.

```sh {"name": "workspace-install-demo", "interactive": true}
# Determine the latest release version
ZHI_TAG=$(curl -sf https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')
echo "Latest version: $ZHI_TAG"

# Install the Vault deployment workspace from the OCI registry
# This downloads the workspace files AND installs any declared plugin dependencies
zhi workspace install "oci://ghcr.io/mrwong99/zhi/zhi-workspace-vault:${ZHI_TAG}" /tmp/zhi-vault-demo

echo ""
echo "--- Installed workspace contents ---"
ls -la /tmp/zhi-vault-demo/
```

That single command:

1. **Pulled** the workspace OCI artifact from `ghcr.io`
2. **Extracted** `zhi.yaml`, templates, config files, and apply scripts
3. **Installed** any plugin dependencies declared in the workspace's lock file

This is the recommended way to distribute and consume zhi workspaces -- no
manual plugin installation, no copy-pasting config files.

```sh {"name": "cleanup-vault-demo", "interactive": true, "excludeFromRunAll": true}
rm -rf /tmp/zhi-vault-demo
```

---

## Step 2: Install the zhi Infrastructure Workspace

Now let's install the workspace that deploys the mirror and marketplace.
This workspace stores its config in the Vault instance from Lesson 3.

```sh {"name": "install-zhi-workspace", "interactive": true}
ZHI_TAG=$(curl -sf https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')

# Install the zhi infrastructure workspace
# --skip-plugins if you already have the vault plugins from Lesson 3
zhi workspace install "oci://ghcr.io/mrwong99/zhi/zhi-workspace-zhi:${ZHI_TAG}" /tmp/zhi-infra

echo ""
echo "Installed plugins:"
zhi plugin list
```

> `zhi workspace install` automatically installs declared plugin dependencies.
> If you already have them (e.g. the Vault store plugins from Lesson 3), the
> existing versions are kept unless the workspace requires a newer version.

---

## Step 3: Login to Vault

The workspace needs to authenticate to Vault to read/write its configuration.
The login happens interactively through the editor -- `zhi edit` provides a
login view where you enter your Vault credentials.

For this tutorial, we'll configure the store auth directly in the workspace config
so it works non-interactively:

```sh {"name": "vault-login", "interactive": true}
cd /tmp/zhi-infra

# Get a Vault token using the admin credentials from Lesson 3
VAULT_TOKEN=$(curl -sf http://127.0.0.1:8200/v1/auth/userpass/login/admin \
  -d '{"password": "tutorial-password-123"}' | jq -r '.auth.client_token')

echo "Got Vault token: ${VAULT_TOKEN:0:10}..."

# Temporarily set the token as an environment variable for the store
export VAULT_TOKEN
```

```sh {"name": "verify-vault-store", "interactive": true}
# Verify we can read from the store
cd /tmp/zhi-infra && zhi list paths
```

You should see the zhi infrastructure config tree, with values persisted in Vault.

---

## Step 4: Configure via the Web UI Marketplace

Instead of configuring via CLI, let's use the Web UI -- which includes a
**Marketplace** tab for browsing and installing plugins visually.

First, set the API key for the marketplace so plugin indexing works:

```sh {"name": "set-marketplace-key", "interactive": true}
cd /tmp/zhi-infra

# Set an API key (format: key=provider)
zhi set zhi-marketplace/api-keys "tutorial-key-123=tutorial"
```

Now launch the Web UI:

```sh {"name": "launch-webui-marketplace", "background": true, "interactive": false}
cd /tmp/zhi-infra && zhi edit --ui webui
```

Open your browser to the address shown (default **http://127.0.0.1:9090**).

**What to explore in the Web UI:**

- **Configuration Tree** (`g t`) -- browse and edit all infrastructure settings
- **Components** (`g c`) -- enable/disable the marketplace and mirror components
- **Validation** (`g v`) -- check for configuration errors
- **Marketplace** (`g m`) -- browse, search, and install plugins from OCI registries
- **Installed Plugins** (`g p`) -- view installed plugins, check for updates

The Marketplace tab replaces manual `curl` calls to the API. It provides:

- Full-text search by name, type, or keyword
- Filtering by plugin type (config, transform, store, ui)
- One-click install directly from the registry
- Version info, ratings, and publisher details

> When you're done exploring, press `Ctrl+C` in the terminal or run the cell below:

```sh {"name": "stop-webui-marketplace", "interactive": true, "excludeFromRunAll": true}
lsof -ti:9090 | xargs -r kill 2>/dev/null
echo "Web UI stopped."
```

---

## Step 5: Deploy the Infrastructure

```sh {"name": "validate-zhi", "interactive": true}
cd /tmp/zhi-infra && zhi validate
```

```sh {"name": "deploy-zhi-infra", "interactive": true}
cd /tmp/zhi-infra

# Export generates docker-compose.yml, mirror policy, and API keys file
zhi export
echo ""
echo "--- Generated docker-compose.yml ---"
cat docker-compose.yml
```

```sh {"name": "apply-zhi-infra", "interactive": true}
cd /tmp/zhi-infra && zhi apply
```

### Verify the services

```sh {"name": "check-services", "interactive": true}
# Wait a moment for containers to start
sleep 3

echo "=== Mirror (port 8080) ==="
curl -sf http://127.0.0.1:8080/.well-known/zhi-marketplace.json 2>/dev/null \
  && echo " responding" || echo "starting..."

echo ""
echo "=== Marketplace (port 8090) ==="
curl -sf http://127.0.0.1:8090/.well-known/zhi-marketplace.json 2>/dev/null \
  && echo " responding" || echo "starting..."
```

---

## Step 6: Install Plugins from the Registry

Now let's install the published zhi plugins from GitHub Container Registry.
You can do this via CLI or through the Web UI Marketplace tab:

### Option A: CLI

```sh {"name": "import-plugins", "interactive": true}
# Determine the latest release version
ZHI_TAG=$(curl -sf https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')
echo "Installing plugins at version: $ZHI_TAG"
echo ""

# Install plugins from the OCI registry
for plugin in \
  zhi-config-pokedex \
  zhi-transform-pokedex \
  zhi-store-json \
  zhi-store-memory \
  zhi-store-mirror \
  zhi-ui-httpapi \
  zhi-ui-mcp-sse \
  zhi-ui-webui; do

  echo -n "  $plugin... "
  zhi plugin install "oci://ghcr.io/mrwong99/zhi/${plugin}:${ZHI_TAG}" --skip-verify 2>&1 \
    | tail -1 && echo "" || echo "(may already be installed)"
done

echo ""
echo "Done! All plugins imported."
```

### Option B: Web UI Marketplace

If you prefer a visual experience, launch `zhi edit --ui webui` and navigate to
the **Marketplace** tab (`g m`). Search for plugins by name or type, and click
**Install** to pull them from the registry.

### Search for plugins via CLI

```sh {"name": "search-plugins", "interactive": true}
# Search the marketplace from the CLI
zhi plugin search store --type store 2>/dev/null || echo "(marketplace may need indexing first)"
```

---

## Step 7: List Installed Plugins

```sh {"name": "list-installed", "interactive": true}
zhi plugin list
```

You now have a fully functional plugin ecosystem:
- **Vault** stores the infrastructure config securely
- **Mirror** caches OCI artifacts locally with policy enforcement
- **Marketplace** provides search and discovery via Web UI and CLI

---

## Checkpoint

```sh {"name": "check-04"}
ERRORS=0

# Check mirror
if curl -sf http://127.0.0.1:8080/.well-known/zhi-marketplace.json > /dev/null 2>&1; then
  echo "✓ Mirror is running"
else
  echo "✗ Mirror is not running"
  ERRORS=$((ERRORS + 1))
fi

# Check marketplace
if curl -sf http://127.0.0.1:8090/.well-known/zhi-marketplace.json > /dev/null 2>&1; then
  echo "✓ Marketplace is running"
else
  echo "✗ Marketplace is not running"
  ERRORS=$((ERRORS + 1))
fi

# Check plugins are installed
PLUGIN_COUNT=$(zhi plugin list 2>/dev/null | wc -l)
if [ "$PLUGIN_COUNT" -gt 5 ]; then
  echo "✓ $PLUGIN_COUNT plugins installed"
else
  echo "⚠ Only $PLUGIN_COUNT plugins found (expected 8+)"
fi

if [ $ERRORS -eq 0 ]; then
  echo ""
  echo "Infrastructure is ready! Moving on to Lesson 5."
fi
```

---

## Cleanup

```sh {"name": "cleanup-04", "interactive": true, "excludeFromRunAll": true}
rm -rf /tmp/zhi-infra
echo "Infrastructure workspace cleaned up."
```

---

## Further Reading

- [Sharing and Registries](../docs/user-guide/sharing-and-registries.md) -- OCI plugin distribution, signing, and updates
- [Plugin Discovery](../docs/user-guide/plugin-discovery.md) -- filesystem-based plugin discovery
- [Enterprise Mirror](../docs/user-guide/enterprise-mirror.md) -- air-gapped OCI mirror for enterprise environments
- [Web UI](../docs/user-guide/web-ui.md) -- browser-based editing and marketplace browsing
- [Marketplace Indexing](../docs/user-guide/marketplace-indexing.md) -- indexing plugins for search and discovery

---

**Next up:** [Lesson 5 - Metadata Labels](05-plugins.md) -- we'll explore metadata labels
that control how plugins interpret configuration values.
