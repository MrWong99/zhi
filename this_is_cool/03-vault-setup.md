# Lesson 3: Deploy Vault

Time to deploy real infrastructure. We'll use zhi's **Vault deployment workspace**
to stand up a HashiCorp Vault server -- and along the way, experience three different
ways to edit configuration.

This lesson has three parts:

1. **Deploy Vault** using the CLI (quick path)
2. **Explore the TUI** -- zhi's terminal editor
3. **Explore the Web UI** -- browser-based editing
4. **Explore MCP + AI** -- let Claude help you configure

---

## Part 1: Deploy Vault via CLI

The `deploy/vault` workspace in the zhi repo is a complete Vault deployment system.
Let's use it.

### Set the admin password

The workspace requires an admin password. Everything else has sensible defaults.

```sh {"name": "set-vault-password", "interactive": true}
cd ../deploy/vault

# Set the admin password (this is for a local dev instance)
zhi set vault-auth/admin-password "tutorial-password-123"
```

### Validate the configuration

```sh {"name": "validate-vault", "interactive": true}
cd ../deploy/vault && zhi validate
```

You should see no blocking errors. The workspace defines validators for everything:
port ranges, auth method names, unseal threshold vs. shares, and more.

### Browse the full config tree

```sh {"name": "list-vault-config", "interactive": true}
cd ../deploy/vault && zhi list paths
```

> **Try it yourself:** Inspect specific values to understand the defaults:

```sh {"name": "explore-vault-values", "interactive": true}
cd ../deploy/vault

echo "=== Deployment Mode ==="
zhi get vault-deploy/mode

echo ""
echo "=== Vault Version ==="
zhi get vault-deploy/vault-version

echo ""
echo "=== Unseal Config ==="
zhi get vault-deploy/unseal-shares
zhi get vault-deploy/unseal-threshold

echo ""
echo "=== Auth Methods ==="
zhi get vault-auth/methods
```

### Export and Apply

This is the moment -- zhi will render all templates and run the bootstrap script.
The script starts Vault in Docker, initializes it, unseals it, creates an admin user,
and revokes the root token.

```sh {"name": "export-vault", "interactive": true}
cd ../deploy/vault && zhi export
echo ""
echo "Generated files:"
ls -la apply.sh docker-compose.yml vault-config.hcl
```

```sh {"name": "apply-vault", "interactive": true}
cd ../deploy/vault && zhi apply
```

> **Important:** The apply script prints **unseal keys** and a **root token** to your
> terminal. In a real deployment, you'd save these securely. For this tutorial, the
> admin user (`admin` / `tutorial-password-123`) is all you need.

### Verify Vault is Running

```sh {"name": "check-vault"}
# Check Vault status
curl -s http://127.0.0.1:8200/v1/sys/health | jq .
```

```sh {"name": "vault-login-test", "interactive": true}
# Login with the admin user we created
curl -s http://127.0.0.1:8200/v1/auth/userpass/login/admin \
  -d '{"password": "tutorial-password-123"}' | jq '.auth.client_token'
```

Vault is running and bootstrapped -- entirely configured and deployed by zhi.

---

## Part 2: The TUI (Terminal UI)

zhi's default editor is a full terminal UI built with Bubbletea. It gives you a
navigable tree view, inline editing, validation dashboard, and more.

```sh {"name": "launch-tui", "interactive": true, "excludeFromRunAll": true}
# Launch the TUI editor
# Navigate with arrow keys, Enter to edit, Tab to switch views
# Press q or Ctrl+C to quit when you're done exploring
cd ../deploy/vault && zhi edit --ui tui
```

**What to try in the TUI:**

- Use `↑`/`↓` to browse the config tree
- Press `Enter` on a value to edit it
- Press `Tab` to switch between views (tree, validation, components, export)
- Try the validation view to see all rules
- Press `c` to manage components (try disabling `vault-secrets`)
- Press `q` to quit

> The TUI requires a terminal with TTY support. If you're running this in a
> non-interactive environment, skip to the Web UI below.

---

## Part 3: The Web UI

The Web UI is a browser-based editor. The Vault workspace is pre-configured to
serve it on port 9090.

```sh {"name": "launch-webui", "background": true, "interactive": false}
# Start the Web UI in the background
# It will auto-open your browser (if supported)
cd ../deploy/vault && zhi edit --ui webui
```

Open your browser to **http://127.0.0.1:9090** (it may open automatically).

**What to try in the Web UI:**

- Browse the config tree on the left sidebar
- Click a value to edit it inline
- Use the keyboard shortcuts: `g t` (tree), `g v` (validation), `g c` (components)
- Navigate to the **Validation** tab to see all rules
- Try **Export** to preview generated files
- Check out the **Marketplace** tab (empty for now -- we'll fill it in Lesson 4!)

> When you're done, press `Ctrl+C` in the terminal to stop the Web UI,
> or use the cell below:

```sh {"name": "stop-webui", "interactive": true, "excludeFromRunAll": true}
# Kill any running zhi edit process on port 9090
lsof -ti:9090 | xargs -r kill 2>/dev/null
echo "Web UI stopped."
```

---

## Part 4: MCP + AI Editing

This is where it gets interesting. zhi can run as an **MCP server**, which means
AI assistants like Claude Code can read, edit, validate, and apply your configuration
through natural language.

### Option A: Claude Code (recommended)

If you have Claude Code installed, register zhi as an MCP server:

```sh {"name": "register-mcp", "interactive": true, "excludeFromRunAll": true}
# Register zhi as an MCP server for this project
claude mcp add zhi \
  --scope project \
  -- zhi edit --ui mcp-stdio --workspace ../deploy/vault/zhi.yaml
```

Then start Claude Code and try prompts like:

- *"Show me the current Vault configuration"*
- *"Change the unseal threshold to 2"*
- *"What auth methods are enabled?"*
- *"Validate the configuration and fix any issues"*
- *"Enable the vault-secrets component and add the transit engine"*

```sh {"name": "launch-claude", "interactive": true, "excludeFromRunAll": true}
# Launch Claude Code (it will see zhi as an MCP tool)
claude
```

### Option B: Any MCP-compatible client

You can also run zhi as an HTTP-based MCP server (SSE transport) for any
MCP-compatible client:

```sh {"name": "launch-mcp-sse", "background": true, "interactive": false, "excludeFromRunAll": true}
# Start MCP SSE server (requires the mcp-sse plugin)
cd ../deploy/vault && zhi edit --ui mcp-sse
```

This serves the MCP protocol over HTTP at `http://127.0.0.1:8091/mcp`.

### What MCP exposes

When running as an MCP server, zhi provides:

| MCP Tool | What it does |
|----------|-------------|
| `reload_tree` | Read the full config tree |
| `set_value` | Update a config value |
| `save` | Persist changes to store |
| `validate` | Run all validators |
| `export` | Render templates |
| `apply` | Run provisioning commands |
| `enable_component` / `disable_component` | Toggle components |
| `store_login` / `store_logout` | Authenticate to the store |
| `marketplace_search` / `marketplace_install` | Search and install plugins |

The AI gets the full config structure with metadata, descriptions, and validation
rules -- so it can make informed suggestions.

```sh {"name": "stop-mcp-sse", "interactive": true, "excludeFromRunAll": true}
# Stop the MCP SSE server
lsof -ti:8091 | xargs -r kill 2>/dev/null
echo "MCP SSE server stopped."
```

---

## Checkpoint

```sh {"name": "check-03"}
ERRORS=0

# Vault should be running
if curl -sf http://127.0.0.1:8200/v1/sys/health > /dev/null 2>&1; then
  echo "✓ Vault is running and healthy"
else
  echo "✗ Vault is not running. Check Docker."
  ERRORS=$((ERRORS + 1))
fi

# Admin login should work
TOKEN=$(curl -sf http://127.0.0.1:8200/v1/auth/userpass/login/admin \
  -d '{"password": "tutorial-password-123"}' | jq -r '.auth.client_token')
if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  echo "✓ Admin login works"
else
  echo "✗ Admin login failed"
  ERRORS=$((ERRORS + 1))
fi

# KV v2 engine should be enabled
MOUNTS=$(curl -sf -H "X-Vault-Token: $TOKEN" http://127.0.0.1:8200/v1/sys/mounts | jq -r 'keys[]')
if echo "$MOUNTS" | grep -q "kv/"; then
  echo "✓ KV v2 secret engine is mounted"
else
  echo "⚠ KV v2 not found. Enable the vault-secrets component and re-apply if needed."
fi

if [ $ERRORS -eq 0 ]; then
  echo ""
  echo "Vault is ready! Moving on to Lesson 4."
fi
```

---

## Further Reading

- [Apply](../docs/user-guide/apply.md) -- apply commands, named targets, and streaming output
- [Web UI](../docs/user-guide/web-ui.md) -- browser-based editing, keyboard shortcuts, TLS
- [Vault Credential Management](../docs/user-guide/vault-credentials.md) -- AppRole and token generation for deployed apps
- [Export and Templates](../docs/user-guide/export-and-templates.md) -- template syntax, component-aware rendering

---

**Next up:** [Lesson 4 - zhi Infrastructure](04-zhi-infrastructure.md) -- we'll deploy
a plugin marketplace and mirror, then import all the published zhi plugins.
