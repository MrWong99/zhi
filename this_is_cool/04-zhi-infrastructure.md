# Lesson 4: zhi Infrastructure

Now that Vault is running, we'll deploy zhi's own infrastructure:

- **zhi-mirror** -- a pull-through cache and policy engine for plugins
- **zhi-marketplace** -- a registry for browsing, searching, and rating plugins

Then we'll populate them with the real plugins published at `ghcr.io/mrwong99/zhi/`.

---

## Step 1: Install the Vault Store Plugin

The zhi infrastructure workspace stores its config *in Vault* (the one we deployed
in Lesson 3). To connect, we need the Vault store plugins.

```sh {"name": "install-vault-plugins", "interactive": true}
# Determine the latest release version
ZHI_TAG=$(curl -sf https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')
echo "Using plugin version: $ZHI_TAG"

# Install the Vault store plugins
zhi plugin install "oci://ghcr.io/mrwong99/zhi/zhi-store-vault:${ZHI_TAG}" --skip-verify
zhi plugin install "oci://ghcr.io/mrwong99/zhi/zhi-store-vault-manager:${ZHI_TAG}" --skip-verify

echo ""
echo "Installed plugins:"
zhi plugin list
```

> We use `--skip-verify` because this is a tutorial. In production, you'd configure
> signing keys and let zhi verify plugin signatures automatically.

---

## Step 2: Login to Vault

The workspace needs to authenticate to Vault to read/write its configuration.
The login happens interactively through the editor -- `zhi edit` provides a
login view where you enter your Vault credentials.

For this tutorial, we'll configure the store auth directly in the workspace config
so it works non-interactively:

```sh {"name": "vault-login", "interactive": true}
cd ../deploy/zhi

# Get a Vault token using the admin credentials from Lesson 3
VAULT_TOKEN=$(curl -sf http://127.0.0.1:8200/v1/auth/userpass/login/admin \
  -d '{"password": "tutorial-password-123"}' | jq -r '.auth.client_token')

echo "Got Vault token: ${VAULT_TOKEN:0:10}..."

# Temporarily set the token as an environment variable for the store
export VAULT_TOKEN
```

```sh {"name": "verify-vault-store", "interactive": true}
# Verify we can read from the store
cd ../deploy/zhi && zhi list paths
```

You should see the zhi infrastructure config tree, with values persisted in Vault.

---

## Step 3: Configure the Infrastructure

Let's review and adjust the settings. The workspace has three components:

```sh {"name": "list-zhi-components", "interactive": true}
cd ../deploy/zhi && zhi component list
```

### Review the defaults

```sh {"name": "explore-zhi-config", "interactive": true}
cd ../deploy/zhi

echo "=== General Settings ==="
zhi get zhi/registry

echo ""
echo "=== Mirror Settings ==="
zhi get zhi-mirror/port
zhi get zhi-mirror/marketplace
zhi get zhi-mirror/signatures

echo ""
echo "=== Marketplace Settings ==="
zhi get zhi-marketplace/port
```

### Set an API key for the marketplace

The marketplace needs at least one API key for plugin indexing.

```sh {"name": "set-marketplace-key", "interactive": true}
cd ../deploy/zhi

# Set an API key (format: key=provider)
zhi set zhi-marketplace/api-keys "tutorial-key-123=tutorial"
```

> Note: `zhi set` automatically persists changes to the store (Vault), so there's
> no separate save step needed.

### Validate

```sh {"name": "validate-zhi", "interactive": true}
cd ../deploy/zhi && zhi validate
```

---

## Step 4: Deploy the Infrastructure

```sh {"name": "deploy-zhi-infra", "interactive": true}
cd ../deploy/zhi

# Export generates docker-compose.yml, mirror policy, and API keys file
zhi export
echo ""
echo "--- Generated docker-compose.yml ---"
cat docker-compose.yml
```

```sh {"name": "apply-zhi-infra", "interactive": true}
cd ../deploy/zhi && zhi apply
```

### Verify the services

```sh {"name": "check-services", "interactive": true}
# Wait a moment for containers to start
sleep 3

echo "=== Mirror (port 8080) ==="
curl -sf http://127.0.0.1:8080/.well-known/zhi-marketplace.json 2>/dev/null \
  && echo " ✓ responding" || echo "starting..."

echo ""
echo "=== Marketplace (port 8090) ==="
curl -sf http://127.0.0.1:8090/.well-known/zhi-marketplace.json 2>/dev/null \
  && echo " ✓ responding" || echo "starting..."
```

---

## Step 5: Populate with Real Plugins

Now the exciting part -- let's import all the published zhi plugins
from GitHub Container Registry into our local mirror.

These are the real plugins from the zhi project:

```sh {"name": "import-plugins", "interactive": true}
# Determine the latest release version
ZHI_TAG=$(curl -sf https://api.github.com/repos/MrWong99/zhi/releases/latest | jq -r '.tag_name')
echo "Installing plugins at version: $ZHI_TAG"
echo ""

# Install each plugin (|| true to continue on already-installed)
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
    | tail -1 && echo "" || echo "⚠"
done

echo ""
echo "Done! All plugins imported."
```

### Index plugins in the marketplace

Now let's tell the marketplace to index these plugins so they're searchable.
The API requires `name`, `type`, and `ociRef` fields:

```sh {"name": "index-marketplace", "interactive": true}
MARKETPLACE="http://127.0.0.1:8090"
API_KEY="tutorial-key-123"

# Index each plugin with the marketplace
for entry in \
  "zhi-config-pokedex config" \
  "zhi-transform-pokedex transform" \
  "zhi-store-json store" \
  "zhi-store-memory store" \
  "zhi-store-vault store" \
  "zhi-store-vault-manager store" \
  "zhi-store-mirror store" \
  "zhi-ui-httpapi ui" \
  "zhi-ui-mcp-sse ui" \
  "zhi-ui-webui ui"; do

  plugin="${entry%% *}"
  ptype="${entry##* }"
  echo -n "Indexing $plugin ($ptype)... "
  curl -sf -X POST "$MARKETPLACE/api/v1/plugins" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"$plugin\", \"type\": \"$ptype\", \"ociRef\": \"ghcr.io/mrwong99/zhi/$plugin\"}" \
    && echo "✓" || echo "⚠ (may already exist)"
done
```

### Search the marketplace

```sh {"name": "search-marketplace", "interactive": true}
# Search for all plugins via the marketplace API
curl -sf "http://127.0.0.1:8090/api/v1/search" | jq '.results[].name'
```

```sh {"name": "search-store-plugins", "interactive": true}
# Search specifically for store plugins
curl -sf "http://127.0.0.1:8090/api/v1/search?q=store&type=store" | jq '.results[].name'
```

> **Try it yourself:** Search for `ui`, `config`, or `transform` plugins!

```sh {"name": "try-search", "interactive": true}
# Your turn!
curl -sf "http://127.0.0.1:8090/api/v1/search?q=ui&type=ui" | jq '.results[].name'
```

---

## Step 6: List Installed Plugins

```sh {"name": "list-installed", "interactive": true}
zhi plugin list
```

You now have a fully functional plugin ecosystem:
- **Vault** stores the infrastructure config securely
- **Mirror** caches OCI artifacts locally with policy enforcement
- **Marketplace** provides search and discovery

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

**Next up:** [Lesson 5 - Plugins](05-plugins.md) -- we'll use the Pokedex plugins to
see config, transforms, and validation in action.
