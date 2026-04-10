# Lesson 2: Your First Workspace

<audio controls src="audio/02-your-first-workspace.mp3">
  Your browser does not support the audio element.
</audio>

In this lesson you'll create a workspace from scratch, explore the config tree,
edit values, trigger validation, and export rendered files. All without any external
services -- just zhi and your terminal.

---

## Step 1: Initialize a Workspace

Let's create a playground workspace in a temporary directory.

```sh {"name": "init-workspace", "interactive": true}
# Create a fresh workspace
mkdir -p /tmp/zhi-playground
cd /tmp/zhi-playground
zhi init
```

```sh {"name": "inspect-workspace"}
# See what zhi init created
ls -la /tmp/zhi-playground/
echo ""
echo "--- zhi.yaml ---"
cat /tmp/zhi-playground/zhi.yaml
```

You should see:
- `zhi.yaml` -- the workspace configuration (a Pokedex example)
- `config/` -- where you define your configuration structure
- `templates/` -- where export templates live
- `app-data/` -- example application data (from the Pokedex demo)
- `.zhi/` -- internal state (component toggles, etc.)

> **Note:** `zhi init` creates a Pokedex example workspace by default. In the next
> step, we'll replace the config and `zhi.yaml` with our own.

---

## Step 2: Define Some Configuration

Let's create a config file that defines a small application's settings.
This is a **structuredfile** config -- plain YAML that zhi reads as a config tree.

Validators use Go code that returns `[]config.ValidationResult`:

```sh {"name": "create-config", "interactive": true}
# Remove the default Pokedex config so only our custom config is loaded
rm -f /tmp/zhi-playground/config/app.yaml

cat > /tmp/zhi-playground/config/app.yml << 'YAML'
app:
  name:
    val: "my-cool-app"
    metadata:
      description: "Application name"
      display-name: "App Name"
    validation: |-
      name, ok := v.Val.(string)
      if !ok || name == "" {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "app name must be a non-empty string",
        }}, nil
      }
      return nil, nil

  port:
    val: 8080
    metadata:
      description: "HTTP port the application listens on"
      display-name: "Port"
    validation: |-
      port, ok := v.Val.(int)
      if !ok {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "port must be a number",
        }}, nil
      }
      if port < 1024 || port > 65535 {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "port must be between 1024 and 65535",
        }}, nil
      }
      return nil, nil

  debug:
    val: false
    metadata:
      description: "Enable debug mode"
      display-name: "Debug Mode"

  log-level:
    val: "info"
    metadata:
      description: "Logging verbosity"
      display-name: "Log Level"
    validation: |-
      level, ok := v.Val.(string)
      if !ok {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "log-level must be a string",
        }}, nil
      }
      allowed := map[string]bool{"trace": true, "debug": true, "info": true, "warn": true, "error": true}
      if !allowed[level] {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "must be one of: trace, debug, info, warn, error",
        }}, nil
      }
      return nil, nil
YAML
echo "Config created!"
```

---

## Step 3: Browse the Config Tree

Now let's see how zhi reads that file.

```sh {"name": "list-tree", "interactive": true}
cd /tmp/zhi-playground && zhi list paths
```

```sh {"name": "get-value", "interactive": true}
# Read a specific value
cd /tmp/zhi-playground && zhi get app/port
```

> **Try it yourself:** Run `zhi get app/name` or `zhi get app/debug` to read other values.

```sh {"name": "try-get", "interactive": true}
# Your turn! Get any value from the tree:
cd /tmp/zhi-playground && zhi get app/name
```

---

## Step 4: Edit Values

You can set values directly from the CLI.

```sh {"name": "set-value", "interactive": true}
cd /tmp/zhi-playground && zhi set app/port 3000
echo ""
echo "New value:"
zhi get app/port
```

### Trigger a Validation Error

Remember the validator on `port`? It requires 1024-65535. Let's break it.

```sh {"name": "set-invalid-value", "interactive": true}
cd /tmp/zhi-playground && zhi set app/port 80
echo ""
echo "Validation results:"
zhi validate
```

You should see a **blocking** validation error. Let's fix it:

```sh {"name": "fix-value", "interactive": true}
cd /tmp/zhi-playground && zhi set app/port 3000
echo ""
zhi validate
```

> **Try it yourself:** Try setting `app/log-level` to an invalid value like `"verbose"`,
> then check `zhi validate`. Fix it afterward!

```sh {"name": "try-validation", "interactive": true}
# Your turn! Try breaking and fixing a value:
cd /tmp/zhi-playground && zhi set app/log-level "verbose"
zhi validate
```

---

## Step 5: Export Templates

Let's create a template that renders our config into a real file.

```sh {"name": "create-template", "interactive": true}
cat > /tmp/zhi-playground/templates/app-config.json.tmpl << 'TMPL'
{
  "name": "{{ .Get "app/name" }}",
  "port": {{ .Get "app/port" }},
  "debug": {{ .Get "app/debug" }},
  "logLevel": "{{ .Get "app/log-level" }}"
}
TMPL

# Register the template in zhi.yaml
cat > /tmp/zhi-playground/zhi.yaml << 'YAML'
version: "1"

config:
  provider: structuredfile
  options:
    directory: ./config

export:
  templates:
    - name: app-config
      template: ./templates/app-config.json.tmpl
      output: ./app-config.json
YAML

echo "Template and workspace config updated!"
```

Now export:

```sh {"name": "export-config", "interactive": true}
cd /tmp/zhi-playground && zhi export
echo ""
echo "--- Generated app-config.json ---"
cat /tmp/zhi-playground/app-config.json
```

You just went from structured config definition to a rendered JSON file.

---

## Step 6: Apply Configuration

Exporting renders config into files -- but what about actually *doing* something
with those files? That's where `zhi apply` comes in.

`zhi apply` runs a shell command you define in `zhi.yaml`. Typically this is
something like `docker compose up -d`, `kubectl apply`, or any provisioning tool.
When `pre-export: true` is set, zhi re-exports all templates before running the command.

Let's add an apply command to our workspace:

```sh {"name": "add-apply", "interactive": true}
cat > /tmp/zhi-playground/zhi.yaml << 'YAML'
version: "1"

config:
  provider: structuredfile
  options:
    directory: ./config

export:
  templates:
    - name: app-config
      template: ./templates/app-config.json.tmpl
      output: ./app-config.json

apply:
  command: "echo 'Configuration applied! Generated app-config.json:' && cat ./app-config.json"
  pre-export: true
  timeout: 30
YAML

echo "Workspace updated with apply command!"
```

Now run it:

```sh {"name": "run-apply", "interactive": true}
cd /tmp/zhi-playground && zhi apply
```

The apply system:

1. Runs `zhi export` first (because `pre-export: true`)
2. Executes the configured `command` in a shell
3. Streams stdout/stderr in real-time
4. Reports the exit code

You can preview what would run without executing:

```sh {"name": "dry-run-apply", "interactive": true}
cd /tmp/zhi-playground && zhi apply --dry-run
```

You can also define **named targets** for different operations (deploy, destroy, restart):

```yaml {"excludeFromRunAll": true}
# Example: named apply targets (don't run this, just read!)
apply:
  default:
    command: "docker compose up -d"
    pre-export: true
  destroy:
    command: "docker compose down -v"
  restart:
    command: "docker compose restart"
```

Run them with `zhi apply destroy` or `zhi apply restart`.

This is the full core loop: **define** -> **edit** -> **validate** -> **export** -> **apply**.

---

## Step 7: Add Components

Components let you group config paths and toggle them on/off.
Let's add a database section and make it optional.

```sh {"name": "add-database-config", "interactive": true}
cat > /tmp/zhi-playground/config/database.yml << 'YAML'
database:
  host:
    val: "localhost"
    metadata:
      description: "Database hostname"
      display-name: "DB Host"

  port:
    val: 5432
    metadata:
      description: "Database port"
      display-name: "DB Port"

  name:
    val: "myapp"
    metadata:
      description: "Database name"
      display-name: "DB Name"
YAML

# Update zhi.yaml with components (keep apply from Step 6)
cat > /tmp/zhi-playground/zhi.yaml << 'YAML'
version: "1"

config:
  provider: structuredfile
  options:
    directory: ./config

components:
  - name: app
    paths: ["app/"]
    mandatory: true

  - name: database
    paths: ["database/"]
    mandatory: false

export:
  templates:
    - name: app-config
      template: ./templates/app-config.json.tmpl
      output: ./app-config.json

apply:
  command: "echo 'Configuration applied!' && cat ./app-config.json"
  pre-export: true
  timeout: 30
YAML

echo "Added database component!"
```

```sh {"name": "list-components", "interactive": true}
cd /tmp/zhi-playground && zhi component list
```

```sh {"name": "toggle-component", "interactive": true}
# Disable the database component
cd /tmp/zhi-playground && zhi component disable database
echo ""
echo "Components after disable:"
zhi component list
echo ""
echo "Tree (database paths should be excluded):"
zhi list paths
```

```sh {"name": "enable-component", "interactive": true}
# Re-enable it
cd /tmp/zhi-playground && zhi component enable database
echo ""
zhi list paths
```

---

## Checkpoint

Let's verify everything works:

```sh {"name": "check-02"}
cd /tmp/zhi-playground
ERRORS=0

# Check config tree has values
if zhi get app/port &>/dev/null; then
  echo "✓ Config tree is readable"
else
  echo "✗ Config tree not readable"
  ERRORS=$((ERRORS + 1))
fi

# Check validation passes (exit code 0 means no blocking errors)
if zhi validate >/dev/null 2>&1; then
  echo "✓ Validation passes"
else
  echo "✗ Validation has blocking errors -- fix them first!"
  ERRORS=$((ERRORS + 1))
fi

# Check export works
if [ -f app-config.json ]; then
  echo "✓ Export file exists"
else
  echo "✗ Export file missing -- run 'zhi export'"
  ERRORS=$((ERRORS + 1))
fi

# Check apply works
if zhi apply 2>&1 | grep -q "exit code 0"; then
  echo "✓ Apply runs successfully"
else
  echo "✗ Apply failed"
  ERRORS=$((ERRORS + 1))
fi

if [ $ERRORS -eq 0 ]; then
  echo ""
  echo "All checks passed! You're ready for Lesson 3."
fi
```

---

## Cleanup

```sh {"name": "cleanup-02", "interactive": true, "excludeFromRunAll": true}
rm -rf /tmp/zhi-playground
echo "Playground cleaned up."
```

---

## Further Reading

- [Getting Started](../docs/user-guide/getting-started.md) -- installation and first workspace
- [Workspace Configuration](../docs/user-guide/workspace-configuration.md) -- full `zhi.yaml` reference
- [Export and Templates](../docs/user-guide/export-and-templates.md) -- template syntax and built-in formats
- [Apply](../docs/user-guide/apply.md) -- apply commands, named targets, and streaming output
- [Components](../docs/user-guide/components.md) -- grouping and toggling config paths
- [Structured File Provider](../docs/plugin-development/structuredfile-provider.md) -- config file format and validation code

---

**Next up:** [Lesson 3 - Deploy Vault](03-vault-setup.md) -- we'll deploy a real HashiCorp
Vault instance and experience zhi's three different editing environments.
