# Lesson 6: Bring It Together

In this final lesson, you'll build a custom workspace from scratch that ties together
everything from the tutorial:

- A **structuredfile config** defining a web application's settings
- The **Vault store** from Lesson 3 for persistent, encrypted storage
- A **template** that generates a docker-compose file
- An **apply command** that deploys it

This is the real zhi workflow: define config -> edit -> validate -> export -> apply.

---

## Step 1: Scaffold the Workspace

```sh {"name": "scaffold-workspace", "interactive": true}
mkdir -p /tmp/zhi-myapp
cd /tmp/zhi-myapp
zhi init
```

---

## Step 2: Define the Configuration

We'll model a simple web application with a database and optional Redis cache.
Validators use Go code that returns `[]config.ValidationResult`.

```sh {"name": "define-config", "interactive": true}
cat > /tmp/zhi-myapp/config/webapp.yml << 'YAML'
webapp:
  name:
    val: "my-webapp"
    metadata:
      description: "Application name"
      display-name: "App Name"
    validation: |-
      name, ok := v.Val.(string)
      if !ok || name == "" {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "app name cannot be empty",
        }}, nil
      }
      return nil, nil

  image:
    val: "nginx:alpine"
    metadata:
      description: "Docker image to deploy"
      display-name: "Docker Image"
    imports:
      - strings
    validation: |-
      image, ok := v.Val.(string)
      if !ok {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "image must be a string",
        }}, nil
      }
      if !strings.Contains(image, ":") {
        return []config.ValidationResult{{
          Severity: config.Warning,
          Message:  "image should include a tag (e.g. nginx:alpine)",
        }}, nil
      }
      return nil, nil

  port:
    val: 8080
    metadata:
      description: "Port exposed to the host"
      display-name: "Host Port"
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

  replicas:
    val: 1
    metadata:
      description: "Number of container replicas"
      display-name: "Replicas"
    validation: |-
      replicas, ok := v.Val.(int)
      if !ok {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "replicas must be a number",
        }}, nil
      }
      if replicas < 1 || replicas > 10 {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "replicas must be between 1 and 10",
        }}, nil
      }
      return nil, nil
YAML

cat > /tmp/zhi-myapp/config/database.yml << 'YAML'
database:
  engine:
    val: "postgres"
    metadata:
      description: "Database engine"
      display-name: "DB Engine"
    validation: |-
      engine, ok := v.Val.(string)
      if !ok {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "engine must be a string",
        }}, nil
      }
      allowed := map[string]bool{"postgres": true, "mysql": true, "mariadb": true}
      if !allowed[engine] {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "must be postgres, mysql, or mariadb",
        }}, nil
      }
      return nil, nil

  version:
    val: "16"
    metadata:
      description: "Database version tag"
      display-name: "DB Version"

  port:
    val: 5432
    metadata:
      description: "Database port (internal)"
      display-name: "DB Port"

  name:
    val: "appdb"
    metadata:
      description: "Database name to create"
      display-name: "DB Name"
    validation: |-
      name, ok := v.Val.(string)
      if !ok || name == "" {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "database name is required",
        }}, nil
      }
      return nil, nil

  password:
    val: ""
    metadata:
      description: "Database root password"
      display-name: "DB Password"
      ui.password: true
      config.required: true
    imports:
      - strings
    validation: |-
      password, ok := v.Val.(string)
      if !ok || strings.TrimSpace(password) == "" {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "database password is required",
        }}, nil
      }
      if len(password) < 8 {
        return []config.ValidationResult{{
          Severity: config.Warning,
          Message:  "password should be at least 8 characters",
        }}, nil
      }
      return nil, nil
YAML

cat > /tmp/zhi-myapp/config/cache.yml << 'YAML'
cache:
  enabled:
    val: false
    metadata:
      description: "Enable Redis cache"
      display-name: "Cache Enabled"

  port:
    val: 6379
    metadata:
      description: "Redis port"
      display-name: "Cache Port"
YAML

echo "Config defined!"
```

---

## Step 3: Wire Up the Workspace

Connect to Vault for storage, define components, and set up templates.

```sh {"name": "configure-workspace", "interactive": true}
cat > /tmp/zhi-myapp/zhi.yaml << 'YAML'
version: "1"

config:
  provider: structuredfile
  options:
    directory: ./config

store:
  provider: vault-manager
  options:
    addr: "http://127.0.0.1:8200"
    mount: "kv"
    prefix: "myapp"
    workspace: "myapp"

ui:
  provider: webui
  options:
    addr: "127.0.0.1:9092"
    auto_open: false

components:
  - name: webapp
    description: "Core web application settings"
    paths: ["webapp/"]
    mandatory: true
  - name: database
    description: "Database configuration"
    paths: ["database/"]
    mandatory: true
  - name: cache
    description: "Optional Redis cache"
    paths: ["cache/"]
    mandatory: false

export:
  templates:
    - name: docker-compose
      template: ./templates/docker-compose.yml.tmpl
      output: ./docker-compose.yml

apply:
  command: "docker compose config"
  workdir: "."
  pre-export: true
  timeout: 30
YAML

echo "Workspace wired up!"
```

---

## Step 4: Create the Docker Compose Template

This template uses Go's `text/template` with zhi's tree functions and conditionals
based on component state.

```sh {"name": "create-compose-template", "interactive": true}
mkdir -p /tmp/zhi-myapp/templates
cat > /tmp/zhi-myapp/templates/docker-compose.yml.tmpl << 'TMPL'
services:
  {{ .Get "webapp/name" }}:
    image: {{ .Get "webapp/image" }}
    ports:
      - "{{ .Get "webapp/port" }}:80"
    deploy:
      replicas: {{ .Get "webapp/replicas" }}
    depends_on:
      - db
{{- if .ComponentEnabled "cache" }}
      - redis
{{- end }}
    environment:
      DATABASE_URL: "{{ .Get "database/engine" }}://app:{{ .Get "database/password" }}@db:{{ .Get "database/port" }}/{{ .Get "database/name" }}"
{{- if .ComponentEnabled "cache" }}
      REDIS_URL: "redis://redis:{{ .Get "cache/port" }}"
{{- end }}

  db:
    image: {{ .Get "database/engine" }}:{{ .Get "database/version" }}
    environment:
      POSTGRES_DB: {{ .Get "database/name" }}
      POSTGRES_PASSWORD: {{ .Get "database/password" }}
    volumes:
      - db-data:/var/lib/postgresql/data
    ports:
      - "{{ .Get "database/port" }}:5432"

{{- if .ComponentEnabled "cache" }}

  redis:
    image: redis:7-alpine
    ports:
      - "{{ .Get "cache/port" }}:6379"
{{- end }}

volumes:
  db-data:
TMPL

echo "Template created!"
```

---

## Step 5: Authenticate and Set Values

The Vault store needs a token for authentication. We'll obtain one from the
admin user created in Lesson 3:

```sh {"name": "login-myapp", "interactive": true}
cd /tmp/zhi-myapp

# Get a Vault token using the admin credentials from Lesson 3
export VAULT_TOKEN=$(curl -sf http://127.0.0.1:8200/v1/auth/userpass/login/admin \
  -d '{"password": "tutorial-password-123"}' | jq -r '.auth.client_token')

echo "Authenticated to Vault (token: ${VAULT_TOKEN:0:10}...)"
```

```sh {"name": "set-myapp-values", "interactive": true}
cd /tmp/zhi-myapp

# Set the required password
zhi set database/password "super-secret-db-pass"

# Customize the app
zhi set webapp/name "cool-webapp"
zhi set webapp/replicas 2

echo ""
echo "Values set and saved to Vault!"
```

> Note: `zhi set` automatically persists changes to the store, so there's no
> separate save step needed.

---

## Step 6: Validate and Export

```sh {"name": "validate-myapp", "interactive": true}
cd /tmp/zhi-myapp && zhi validate
```

```sh {"name": "export-myapp", "interactive": true}
cd /tmp/zhi-myapp && zhi export

echo ""
echo "--- Generated docker-compose.yml ---"
cat docker-compose.yml
```

Notice how the generated file only includes the services for enabled components.

---

## Step 7: Toggle the Cache Component

```sh {"name": "enable-cache", "interactive": true}
cd /tmp/zhi-myapp

echo "=== Before (no cache) ==="
zhi component list
echo ""

# Enable the cache component
zhi component enable cache

echo ""
echo "=== After (with cache) ==="
zhi component list

# Re-export to see the difference
zhi export
echo ""
echo "--- Updated docker-compose.yml ---"
cat docker-compose.yml
```

The Redis service now appears in the compose file, and the webapp gets a `REDIS_URL`
environment variable. This is the power of **component-driven configuration**.

---

## Step 8: Apply (Dry Run)

Our apply command is set to `docker compose config` -- a dry run that validates
the generated compose file without starting anything.

```sh {"name": "apply-myapp", "interactive": true}
cd /tmp/zhi-myapp && zhi apply
```

If you wanted to actually deploy, you'd change the apply command to
`docker compose up -d` in `zhi.yaml`.

---

## Step 9: Verify Vault Persistence

Your configuration is stored in Vault. Let's prove it by reading directly:

```sh {"name": "verify-vault-persistence", "interactive": true}
# Get a Vault token
TOKEN=$(curl -sf http://127.0.0.1:8200/v1/auth/userpass/login/admin \
  -d '{"password": "tutorial-password-123"}' | jq -r '.auth.client_token')

# List stored keys under the myapp prefix
echo "=== Stored keys in Vault ==="
curl -sf -H "X-Vault-Token: $TOKEN" \
  "http://127.0.0.1:8200/v1/kv/metadata/myapp?list=true" | jq '.data.keys'
```

Your configuration lives securely in Vault, versioned and encrypted at rest.
The `vault-manager` store provider handles tree serialization and versioning
internally.

---

## What You've Built

```text {"excludeFromRunAll": true}
┌─────────────────────────────────────────────────────────┐
│                    Your Setup                           │
│                                                         │
│  ┌──────────────┐      ┌───────────────┐                │
│  │ Your Custom  │─────▶│ HashiCorp     │                │
│  │ Workspace    │ store│ Vault         │                │
│  │ (myapp)      │◀─────│ (Lesson 3)    │                │
│  └──────┬───────┘      └───────────────┘                │
│         │ export                                        │
│         ▼                                               │
│  ┌──────────────┐      ┌───────────────┐                │
│  │ docker-      │      │ Marketplace   │                │
│  │ compose.yml  │      │ + Mirror      │                │
│  └──────────────┘      │ (Lesson 4)    │                │
│                        └───────────────┘                │
└─────────────────────────────────────────────────────────┘
```

You've completed the full zhi workflow:

1. **Defined** a config structure with types, defaults, and validators
2. **Edited** values through CLI, TUI, Web UI, and MCP
3. **Stored** configuration securely in Vault
4. **Exported** templates that render into deployment files
5. **Applied** provisioning commands
6. **Used components** to toggle optional features
7. **Managed plugins** through a local marketplace and mirror

---

## Cleanup

```sh {"name": "cleanup-06", "interactive": true, "excludeFromRunAll": true}
# Remove the workspace
rm -rf /tmp/zhi-myapp

echo "Workspace cleaned up."
echo ""
echo "To tear down ALL tutorial infrastructure (Vault, mirror, marketplace):"
echo "  Run the cleanup cell in README.md"
```

---

## Where to Go From Here

- **Build your own config plugin** -- see [Plugin Development docs](../docs/plugin-development/overview.md)
- **Write a transform plugin** -- add validation rules, computed fields, or policy enforcement
- **Create a workspace package** -- publish it with `zhi workspace publish` for others to install
- **Set up for production** -- enable TLS, configure real auth methods, set up mirror policies
- **Use zhi with AI** -- register as an MCP server and let Claude manage your infrastructure

Happy configuring!
