# Lesson 5: Metadata Labels and the Java Plugin

Every configuration value in zhi can carry **metadata labels** -- semantic
annotations that tell plugins how to interpret, display, and handle that value.
Labels follow a namespace convention (`<namespace>.<name>`) and are the primary
way to communicate intent across plugin boundaries.

In this lesson you'll:

1. Discover available labels via the CLI
2. Use labels in a structuredfile config to control UI behavior, store handling, and validation
3. See how the Java config plugin uses Bean Validation annotations to achieve the same thing

---

## Step 1: Discover Available Labels

zhi ships with a built-in label registry. Let's explore it.

```sh {"name": "list-all-labels", "interactive": true}
# List every registered metadata label, grouped by namespace
zhi labels list
```

Labels are organized into namespaces that correspond to plugin types:

| Namespace | Interpreted by | Purpose |
|-----------|---------------|---------|
| `ui.*` | UI plugins (TUI, Web UI) | Control how values are displayed and edited |
| `store.*` | Store plugins (Vault, JSON) | Control persistence behavior |
| `transform.*` | Transform plugins | Control which transforms apply |
| `config.*` | Config plugins | Mark values as required, immutable, etc. |
| `core.*` | The engine itself | Descriptions, types, deprecation notices |

### Filter by namespace

```sh {"name": "list-ui-labels", "interactive": true}
zhi labels list --namespace ui
```

```sh {"name": "list-store-labels", "interactive": true}
zhi labels list --namespace store
```

### Get detailed info about a label

```sh {"name": "label-info-password", "interactive": true}
zhi labels info ui.password
```

```sh {"name": "label-info-writeonly", "interactive": true}
zhi labels info store.writeonly
```

> **Try it yourself:** Run `zhi labels info config.required` or `zhi labels info ui.readonly`
> to see their descriptions, types, defaults, and examples.

```sh {"name": "try-label-info", "interactive": true}
# Your turn!
zhi labels info config.required
```

---

## Step 2: Use Labels in a Structuredfile Config

Let's create a workspace that uses metadata labels to control how the UI
renders each field and how the store handles sensitive values.

```sh {"name": "create-labels-workspace", "interactive": true}
mkdir -p /tmp/zhi-labels/config /tmp/zhi-labels/templates
cd /tmp/zhi-labels && zhi init --force
# Remove default Pokedex config so only our labeled config is loaded
rm -f /tmp/zhi-labels/config/app.yaml
```

```sh {"name": "define-labeled-config", "interactive": true}
cat > /tmp/zhi-labels/config/service.yml << 'YAML'
service:
  name:
    val: "my-api"
    metadata:
      description: "Service name used in deployment"
      display-name: "Service Name"
      config.required: true
      ui.pattern: "^[a-z][a-z0-9-]*$"
      ui.placeholder: "e.g. my-api"
    validation: |-
      name, ok := v.Val.(string)
      if !ok || name == "" {
        return []config.ValidationResult{{
          Severity: config.Blocking,
          Message:  "service name is required",
        }}, nil
      }
      return nil, nil

  port:
    val: 8080
    metadata:
      description: "Port the service listens on"
      display-name: "Port"
      core.type: "port"
      ui.order: 2
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

  api-key:
    val: ""
    metadata:
      description: "API key for external service"
      display-name: "API Key"
      config.required: true
      ui.password: true
      store.writeonly: true

  admin-email:
    val: ""
    metadata:
      description: "Admin contact email"
      display-name: "Admin Email"
      core.type: "email"
      ui.pattern: "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$"
      ui.placeholder: "admin@example.com"

  log-level:
    val: "info"
    metadata:
      description: "Logging verbosity"
      display-name: "Log Level"
      ui.enum:
        - trace
        - debug
        - info
        - warn
        - error

  notes:
    val: ""
    metadata:
      description: "Deployment notes (supports multiple lines)"
      display-name: "Notes"
      ui.multiline: true

  version:
    val: "1.0.0"
    metadata:
      description: "Current deployed version (set by CI)"
      display-name: "Version"
      ui.readonly: true
      config.immutable: true

  cache-ttl:
    val: 300
    metadata:
      description: "Cache time-to-live"
      display-name: "Cache TTL"
      core.unit: "seconds"
      store.ttl: 3600
YAML

echo "Labeled config created!"
```

### See how labels affect the config tree

```sh {"name": "list-labeled-tree", "interactive": true}
cd /tmp/zhi-labels && zhi list paths
```

```sh {"name": "get-labeled-values", "interactive": true}
cd /tmp/zhi-labels

echo "=== Service Name ==="
zhi get service/name

echo ""
echo "=== API Key (write-only, password-masked) ==="
zhi get service/api-key

echo ""
echo "=== Version (read-only, immutable) ==="
zhi get service/version
```

### Observe labels in the Web UI

Labels truly shine in the Web UI. Launch it and see how each label changes the rendering:

```sh {"name": "launch-labels-webui", "background": true, "interactive": false}
cd /tmp/zhi-labels && zhi edit --ui webui
```

Open **http://127.0.0.1:8080** and observe:

- `service/api-key` shows a **password input** (masked characters) thanks to `ui.password: true`
- `service/version` is **grayed out / read-only** thanks to `ui.readonly: true`
- `service/log-level` shows a **dropdown** thanks to `ui.enum`
- `service/notes` shows a **textarea** thanks to `ui.multiline: true`
- `service/name` validates against a **regex pattern** (`ui.pattern`)

```sh {"name": "stop-labels-webui", "interactive": true, "excludeFromRunAll": true}
lsof -ti:8080 | xargs -r kill 2>/dev/null
echo "Web UI stopped."
```

---

## Step 3: Label Effects Summary

Here's what each label does when applied to a configuration value:

### UI Labels (interpreted by TUI and Web UI)

| Label | Type | Effect |
|-------|------|--------|
| `ui.readonly` | bool | Field is visible but not editable |
| `ui.password` | bool | Input is masked (dots/asterisks) |
| `ui.hidden` | bool | Field is completely hidden from the UI |
| `ui.multiline` | bool | Uses a textarea for input |
| `ui.pattern` | string | Regex pattern for input validation |
| `ui.enum` | string[] | Restricts input to a dropdown of allowed values |
| `ui.order` | int | Controls display order (lower = first) |
| `ui.group` | string | Groups related fields under a collapsible section |
| `ui.section` | string | Assigns value to a collapsible section |
| `ui.placeholder` | string | Placeholder text in empty input fields |
| `ui.confirm` | bool | Requires confirmation before changing |
| `ui.displayName` | string | Human-readable display name in the UI |
| `ui.format` | string | Hints at display format (e.g., `json`, `yaml`) |
| `ui.showIf` | string | Conditionally shows value based on another config value |
| `ui.yamlSchema` | string | Description of expected YAML structure |
| `ui.listItemPlaceholder` | string | Placeholder for list-type value items |
| `ui.mapKeyPlaceholder` | string | Placeholder for map key input fields |
| `ui.mapValuePlaceholder` | string | Placeholder for map value input fields |

### Store Labels (interpreted by store plugins)

| Label | Type | Effect |
|-------|------|--------|
| `store.writeonly` | bool | Value can be written but never read back |
| `store.encrypt` | bool | Forces encryption even if store-wide encryption is off |
| `store.noversion` | bool | Excludes from version history |
| `store.maxversions` | int | Maximum number of versions to retain |
| `store.ttl` | int | Auto-deletes after N seconds |
| `store.ephemeral` | bool | In-memory only, never persisted |

### Config Labels (interpreted by config plugins)

| Label | Type | Effect |
|-------|------|--------|
| `config.required` | bool | Validation fails if value is empty |
| `config.immutable` | bool | Value cannot be changed after initial set |
| `config.default` | any | Default value if not set |
| `config.env` | string | Environment variable that can override this value |

### Transform Labels

| Label | Type | Effect |
|-------|------|--------|
| `transform.hidden` | bool | Transform plugins cannot access this value |
| `transform.order` | int | Override the default transform order for this value |
| `transform.skip` | string[] | List of transform plugin names to skip |

---

## Step 4: The Java Config Plugin -- Labels via Bean Validation

The `zhi-config-javabean` example shows a different approach: instead of writing
labels in YAML, you define your config as a **Java bean** with Jakarta Bean
Validation annotations. The annotations map to zhi labels and validation rules.

Here's the `DatabaseConfig` bean from the example:

```java {"excludeFromRunAll": true}
// This is the actual code from examples/zhi-config-javabean/
// Don't run this -- just read!

@ConfigPrefix("database")
@SslRequiredForRemoteHost              // custom cross-value constraint
public class DatabaseConfig {

    @NotBlank(message = "database host is required")
    @ConfigProperty(description = "Database server hostname")
    private String host = "localhost";

    @Min(value = 1, message = "port must be at least 1")
    @Max(value = 65535, message = "port must be at most 65535")
    @ConfigProperty(description = "Database server port")
    private int port = 5432;

    @NotBlank(message = "database name is required")
    @ConfigProperty(description = "Name of the database")
    private String name = "myapp";

    @NotBlank(message = "username is required")
    @ConfigProperty(description = "Database login user")
    private String username = "admin";

    @Min(value = 1, message = "max connections must be at least 1")
    @Max(value = 1000, message = "max connections must be at most 1000")
    @ConfigProperty(path = "max-connections",
                    description = "Maximum number of database connections")
    private int maxConnections = 10;

    @ConfigProperty(path = "ssl-enabled",
                    description = "Whether TLS is used for the database connection")
    private boolean sslEnabled = false;
}
```

### How Java annotations map to zhi concepts

| Java Annotation | zhi Equivalent |
|----------------|----------------|
| `@ConfigPrefix("database")` | Path prefix -- all fields become `database/host`, `database/port`, etc. |
| `@ConfigProperty(description="...")` | `metadata.description` label |
| `@ConfigProperty(path="ssl-enabled")` | Overrides the default kebab-case path segment |
| `@NotBlank` | Similar to `config.required: true` + a blocking validation |
| `@Min(1)` / `@Max(65535)` | Blocking validation on numeric range |
| `@SslRequiredForRemoteHost` | Cross-value validation -- warns if host is remote but SSL is off |

### Cross-value validation

The `@SslRequiredForRemoteHost` annotation is a custom class-level constraint:

```java {"excludeFromRunAll": true}
// Custom validator that checks two fields together
public class SslRequiredForRemoteHostValidator
        implements ConstraintValidator<SslRequiredForRemoteHost, DatabaseConfig> {
    @Override
    public boolean isValid(DatabaseConfig cfg, ConstraintValidatorContext ctx) {
        if (cfg == null) return true;
        boolean isLocal = "localhost".equals(cfg.getHost());
        if (isLocal || cfg.isSslEnabled()) return true;

        // SSL should be enabled for remote hosts
        ctx.disableDefaultConstraintViolation();
        String msg = "SSL should be enabled for remote host '" + cfg.getHost() + "'";
        ctx.buildConstraintViolationWithTemplate(msg)
                .addPropertyNode("sslEnabled").addConstraintViolation();
        return false;
    }
}
```

Because it's annotated with `@WarnConstraint`, violations are reported as
**Warning** (not Blocking) -- the zhi host shows them as advisory messages.

### Building and running the Java plugin

The plugin is built with GraalVM `native-image` for a zero-dependency binary:

```sh {"name": "show-java-plugin-build", "excludeFromRunAll": true, "interactive": true}
echo "To build the Java plugin (requires JDK 21+ and GraalVM native-image):"
echo ""
echo "  cd examples/zhi-config-javabean"
echo "  ./gradlew nativeCompile"
echo "  cp build/native/nativeCompile/zhi-config-javabean ~/.zhi/plugins/"
echo ""
echo "Then use it in zhi.yaml:"
echo ""
echo "  config:"
echo '    provider: javabean'
```

The resulting binary follows the standard naming convention (`zhi-config-javabean`),
so zhi discovers it automatically from `~/.zhi/plugins/`.

---

## Step 5: Compare Approaches

Both the YAML structuredfile and the Java plugin achieve the same goal --
defining typed, validated, labeled configuration. Choose based on your needs:

| Aspect | Structuredfile (YAML) | Java Bean |
|--------|----------------------|-----------|
| **Language** | YAML + inline Go validation | Java + Bean Validation |
| **Labels** | Set directly in `metadata` map | Mapped from annotations |
| **Validation** | Inline Go code (Yaegi interpreter) | Jakarta Bean Validation (`@NotBlank`, `@Min`, etc.) |
| **Cross-value checks** | Access `tree` parameter in validation code | Class-level custom constraints |
| **Build** | No build step (plain files) | Gradle + GraalVM native-image |
| **Best for** | Quick configs, ops teams | Type-safe Java ecosystems, enterprise |

---

## Checkpoint

```sh {"name": "check-05"}
cd /tmp/zhi-labels 2>/dev/null || exit 1
ERRORS=0

# Check labels CLI works
LABEL_COUNT=$(zhi labels list 2>/dev/null | grep -c "^" || true)
if [ "$LABEL_COUNT" -gt 10 ]; then
  echo "✓ Label registry has $LABEL_COUNT entries"
else
  echo "✗ Label registry seems empty"
  ERRORS=$((ERRORS + 1))
fi

# Check labeled config loads
if zhi list paths 2>/dev/null | grep -q "service/name"; then
  echo "✓ Labeled config tree loads correctly"
else
  echo "✗ Config tree not loading"
  ERRORS=$((ERRORS + 1))
fi

# Validate (expect blocking errors since api-key and admin-email are empty)
if zhi validate >/dev/null 2>&1; then
  echo "✓ Validation passes"
else
  echo "⚠ Validation has blocking errors (expected -- api-key and admin-email are empty)"
fi

if [ $ERRORS -eq 0 ]; then
  echo ""
  echo "Labels and validation working! On to the final lesson."
fi
```

---

## Cleanup

```sh {"name": "cleanup-05", "interactive": true, "excludeFromRunAll": true}
rm -rf /tmp/zhi-labels
echo "Labels workspace cleaned up."
```

---

## Further Reading

- [Metadata Labels API Design](../docs/design/metadata-labels-api.md) -- full design document for the label registry
- [Structured File Provider](../docs/plugin-development/structuredfile-provider.md) -- config file format and validation code
- [Java Plugin Development](../docs/plugin-development/java-plugin.md) -- Gradle setup, Bean Validation, GraalVM native-image
- [Plugin Development Overview](../docs/plugin-development/overview.md) -- building plugins in Go or other languages

---

**Next up:** [Lesson 6 - Bring It Together](06-bring-it-together.md) -- we'll build
a custom workspace backed by Vault, completing the full zhi workflow.
