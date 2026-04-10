# Lesson 5: Metadata Labels

Metadata labels are semantic annotations on configuration values that control how plugins interpret and handle those values. They influence UI rendering, store behavior, transform processing, and more. This lesson explains what labels are, lists every built-in label, demonstrates their effects, and walks through a concrete Java config plugin that uses them.

## Prerequisites

- Completed [Lesson 1](lesson-01-getting-started.md) through [Lesson 4](lesson-04-workspaces-and-marketplace.md)
- For the Java plugin section: JDK 21+ and Gradle 8.x (optional -- you can follow along without building)

## What Are Metadata Labels?

Every configuration value in zhi can carry a `metadata` map -- a set of key-value pairs that provide hints to plugins and the engine. Metadata labels are the standardized keys in this map. They follow a namespace convention:

```
<namespace>.<name>
```

For example, `ui.password` tells UI plugins to mask the input field, while `store.writeonly` tells store plugins not to return the value when reading.

Labels are set on individual configuration values, either in YAML config files or programmatically in a plugin.

### Discovering Available Labels

List all registered labels from the command line:

```sh
zhi labels list
```

Output (abbreviated):

```
CONFIG namespace:
NAME              TYPE    DEFAULT  DESCRIPTION
config.required   bool    false    Value must be present and non-empty.
config.immutable  bool    false    Value cannot be changed after initial creation.
config.env        string  -        Environment variable name that can override this value.
config.default    any     -        Default value to use if the configuration value is not set.

CORE namespace:
NAME              TYPE    DEFAULT  DESCRIPTION
core.description  string  -        Human-readable description of what this configuration value does.
core.type         string  -        Semantic type hint for the value.
core.deprecated   string  -        Marks this configuration value as deprecated with migration guidance.
core.doc          string  -        URL to documentation for this configuration value.
...
```

Filter by namespace:

```sh
zhi labels list --namespace ui
zhi labels list --namespace store
```

Get detailed information about a specific label:

```sh
zhi labels info ui.password
```

Output:

```
Name:        ui.password
Namespace:   ui
Type:        bool
Description: Masks input characters during entry (displays as dots or asterisks).
             The stored value is unaffected.
Default:     false
Applies to:  ui

Examples:
  true
    Mask password input

Since: 0.1.0
```

## Built-in Labels Reference

### UI Labels

These labels control how values are displayed and edited in the TUI and web UI.

| Label | Type | Default | Effect |
|-------|------|---------|--------|
| `ui.readonly` | bool | false | Value is displayed but cannot be edited |
| `ui.password` | bool | false | Input is masked (dots/asterisks) |
| `ui.hidden` | bool | false | Value is completely hidden from the UI |
| `ui.multiline` | bool | false | Uses a textarea for editing |
| `ui.pattern` | string | -- | Regex pattern for input validation |
| `ui.order` | int | 0 | Display order (lower numbers first) |
| `ui.group` | string | -- | Groups related values together |
| `ui.section` | string | -- | Assigns to a collapsible section |
| `ui.displayName` | string | -- | Human-readable display name |
| `ui.placeholder` | string | -- | Placeholder text in empty fields |
| `ui.enum` | string[] | -- | Restricts input to a list of allowed values |
| `ui.confirm` | bool | -- | Requires confirmation before changing |
| `ui.showIf` | string | -- | Conditionally shows based on another value (format: `path=value`) |
| `ui.format` | string | -- | Display format hint (e.g., `json`, `yaml`) |
| `ui.listItemPlaceholder` | string | -- | Placeholder text for items in list-type values |
| `ui.mapKeyPlaceholder` | string | -- | Placeholder text for key fields in map-type values |
| `ui.mapValuePlaceholder` | string | -- | Placeholder text for value fields in map-type values |
| `ui.yamlSchema` | string | -- | Description of expected YAML structure for multiline YAML values |

### Store Labels

These labels control how values are persisted.

| Label | Type | Default | Effect |
|-------|------|---------|--------|
| `store.writeonly` | bool | false | Value can be written but not read back (for secrets) |
| `store.encrypt` | bool | false | Forces per-value encryption |
| `store.noversion` | bool | false | Excludes from version history |
| `store.ttl` | int | -- | Time-to-live in seconds |
| `store.ephemeral` | bool | false | In-memory only, never persisted |
| `store.maxversions` | int | -- | Maximum version history to retain |

### Transform Labels

These labels control how transform plugins process values.

| Label | Type | Default | Effect |
|-------|------|---------|--------|
| `transform.hidden` | bool | false | Transform plugins cannot access this value |
| `transform.skip` | string[] | -- | List of transform plugins to skip |
| `transform.order` | int | -- | Override default transform ordering |

### Config Labels

These labels affect how the config provider handles values.

| Label | Type | Default | Effect |
|-------|------|---------|--------|
| `config.required` | bool | false | Value must be present and non-empty |
| `config.immutable` | bool | false | Cannot be changed after initial creation |
| `config.env` | string | -- | Environment variable that can override this value |
| `config.default` | any | -- | Default value when not explicitly set |

### Core Labels

These labels are interpreted by the engine and shared across plugin types.

| Label | Type | Default | Effect |
|-------|------|---------|--------|
| `core.description` | string | -- | Human-readable description |
| `core.type` | string | -- | Semantic type hint (e.g., `email`, `url`, `port`, `bool`, `map`, `list`, `yaml`) |
| `core.deprecated` | string | -- | Deprecation message with migration guidance |
| `core.doc` | string | -- | URL to external documentation |
| `core.example` | any | -- | Example value showing expected format |
| `core.unit` | string | -- | Unit of measurement (e.g., `seconds`, `bytes`) |

## Using Labels in Configuration Files

Labels are set in the `metadata` section of a value in your structured file configuration:

```yaml
# config/database.yml
database:
  host:
    val: "localhost"
    metadata:
      core.description: "Database server hostname"
      ui.placeholder: "db.example.com"
      config.required: true

  port:
    val: 5432
    metadata:
      core.description: "Database server port"
      core.type: "port"
      ui.placeholder: "5432"

  password:
    val: ""
    metadata:
      core.description: "Database password"
      ui.password: true
      store.writeonly: true
      config.required: true

  ssl-enabled:
    val: false
    metadata:
      core.description: "Whether TLS is used for database connections"
      ui.displayName: "SSL Enabled"
      ui.section: "Security"

  connection-string:
    val: ""
    metadata:
      core.deprecated: "Use database/host and database/port instead"
      ui.readonly: true
```

When you open this workspace in the web UI (`zhi edit --ui webui`), you will see:

- The password field masked with dots
- Required fields marked as such
- The deprecated value shown with a warning
- Fields grouped by section
- Placeholder text in empty fields

## Writing a Java Config Plugin with Labels

To demonstrate labels from the plugin developer perspective, here is a walkthrough of a Java config plugin that sets metadata labels on its configuration values. This example uses Jakarta Bean Validation for input validation and zhi metadata labels for UI and store behavior.

### Project Structure

```
zhi-config-myapp/
  build.gradle.kts
  src/main/java/com/example/zhiplugin/
    Main.java
    ConfigServiceImpl.java
    model/
      AppConfig.java
```

### The Configuration Bean

The central class is a Java bean where each field maps to a zhi configuration path. Metadata labels are set programmatically when returning values via gRPC.

```java
package com.example.zhiplugin.model;

import dev.zhi.plugin.javabean.annotation.ConfigPrefix;
import dev.zhi.plugin.javabean.annotation.ConfigProperty;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;

@ConfigPrefix("myapp")
public class AppConfig {

    @NotBlank(message = "application name is required")
    @ConfigProperty(description = "Application display name")
    private String name = "My App";

    @Min(value = 1024, message = "port must be at least 1024")
    @Max(value = 65535, message = "port must be at most 65535")
    @ConfigProperty(description = "HTTP listen port")
    private int port = 8080;

    @ConfigProperty(path = "admin-password", description = "Admin password for the dashboard")
    private String adminPassword = "";

    @ConfigProperty(path = "api-key", description = "External API key (write-only)")
    private String apiKey = "";

    @ConfigProperty(path = "log-level", description = "Application log level")
    private String logLevel = "info";

    @ConfigProperty(path = "legacy-mode", description = "Legacy compatibility mode")
    private boolean legacyMode = false;

    // Getters omitted for brevity
}
```

### Setting Labels in the gRPC Service

The `ConfigServiceImpl` controls what metadata is returned for each path. This is where labels are applied:

```java
@Override
public void get(GetRequest req, StreamObserver<GetResponse> obs) {
    String path = req.getPath();
    BeanReflector.ValueEntry entry = reflector.getValue(path);
    if (entry == null) {
        obs.onNext(GetResponse.newBuilder().setFound(false).build());
        obs.onCompleted();
        return;
    }

    // Start with the base metadata from the reflector
    Map<String, Object> metadata = new LinkedHashMap<>();
    if (entry.metadataJson() != null) {
        metadata.putAll(parseJson(entry.metadataJson()));
    }

    // Add labels based on the path
    switch (path) {
        case "myapp/admin-password":
            metadata.put("ui.password", true);        // mask input
            metadata.put("store.writeonly", true);     // never read back
            metadata.put("config.required", true);     // must be set
            break;

        case "myapp/api-key":
            metadata.put("ui.password", true);         // mask input
            metadata.put("store.writeonly", true);      // never read back
            metadata.put("store.encrypt", true);        // force encryption
            break;

        case "myapp/port":
            metadata.put("core.type", "port");         // semantic type
            metadata.put("ui.placeholder", "8080");    // hint
            break;

        case "myapp/log-level":
            metadata.put("ui.enum", List.of(           // dropdown
                "trace", "debug", "info", "warn", "error"
            ));
            break;

        case "myapp/legacy-mode":
            metadata.put("core.deprecated",            // deprecation notice
                "Legacy mode will be removed in v3.0. Migrate to the new API.");
            metadata.put("ui.confirm", true);          // require confirmation
            break;
    }

    String metaJson = toJson(metadata);
    obs.onNext(GetResponse.newBuilder()
        .setFound(true)
        .setValueJson(ByteString.copyFromUtf8(entry.valueJson()))
        .setMetadataJson(ByteString.copyFromUtf8(metaJson))
        .build());
    obs.onCompleted();
}
```

### What the Labels Achieve

| Path | Labels | Effect |
|------|--------|--------|
| `myapp/admin-password` | `ui.password`, `store.writeonly`, `config.required` | Masked in UI, never returned from store, validation fails if empty |
| `myapp/api-key` | `ui.password`, `store.writeonly`, `store.encrypt` | Masked, write-only, individually encrypted |
| `myapp/port` | `core.type: port`, `ui.placeholder` | Shown as a port number with placeholder hint |
| `myapp/log-level` | `ui.enum` | Rendered as a dropdown with fixed choices |
| `myapp/legacy-mode` | `core.deprecated`, `ui.confirm` | Shown with deprecation warning, toggle requires confirmation |

### The go-plugin Handshake (Java)

The `Main.java` follows the standard pattern for Java plugins. The plugin must verify the magic cookie, start a gRPC server, print the handshake line, and watch stdin for EOF:

```java
public class Main {
    public static void main(String[] args) throws Exception {
        // 1. Verify magic cookie
        if (!"zhiplugin-v1".equals(System.getenv("ZHI_PLUGIN"))) {
            System.err.println("This binary is a zhi plugin. Do not run directly.");
            System.exit(1);
        }

        // 2. Start gRPC server
        var reflector = new BeanReflector<>(AppConfig.class, new AppConfig());
        int port = findFreePort();
        Server server = ServerBuilder.forPort(port)
                .addService(new ConfigServiceImpl(reflector))
                .addService(new HealthStatusManager().getHealthService())
                .build().start();

        // 3. Print handshake line
        System.out.printf("1|1|tcp|127.0.0.1:%d|grpc%n", port);
        System.out.flush();

        // 4. Watch stdin for EOF
        new Thread(() -> {
            try { while (System.in.read() != -1); }
            catch (IOException ignored) {}
            server.shutdown();
        }, "stdin-watcher").start();

        server.awaitTermination();
    }
}
```

### Using Labels in Go Plugins

If you are writing a Go plugin instead of Java, the `pkg/zhiplugin/labels` package provides a builder API and convenience helpers:

```go
import "github.com/MrWong99/zhi/pkg/zhiplugin/labels"

// Building metadata with the Builder API
metadata := labels.NewBuilder().
    Password().
    Writeonly().
    Required().
    Description("Database admin password").
    Build()

// Reading labels with helper functions
if labels.IsPassword(value.Metadata) {
    renderMaskedInput(value)
}

if labels.IsWriteonly(value.Metadata) {
    // Don't return the actual value
}

pattern := labels.GetPattern(value.Metadata)
if pattern != "" {
    validateAgainstRegex(value, pattern)
}
```

## Custom Labels

Plugin developers can register custom labels in their own namespace. The convention is to use a namespace that matches your plugin or organization name:

```go
labels.DefaultRegistry.MustRegister(&labels.Label{
    Name:        "mycompany.priority",
    Namespace:   "mycompany",
    Description: "Processing priority for this value.",
    ValueType:   "int",
    DefaultValue: 0,
    Constraints: &labels.LabelConstraints{
        Min: ptr(0.0),
        Max: ptr(10.0),
    },
})
```

Custom labels appear in `zhi labels list` alongside the built-in ones.

## Summary

In this lesson you learned:

- What metadata labels are and how they follow the `namespace.name` convention
- How to discover labels with `zhi labels list` and `zhi labels info`
- Every built-in label across the UI, store, transform, config, and core namespaces
- How to set labels in YAML configuration files
- How to set labels programmatically in a Java config plugin
- The effect each label has on UI rendering, store behavior, and validation
- How to register custom labels for your own plugins

## Further Reading

- [Metadata Labels API Design](../design/metadata-labels-api.md) -- design document with full registry specification and gRPC protocol
- [Java Plugin Development](../plugin-development/java-plugin.md) -- complete Java plugin guide (Gradle, Bean Validation, GraalVM native-image)
- [Config Plugin API](../plugin-development/config-plugin.md) -- the config plugin interface and gRPC layer
- [Plugin Development Overview](../plugin-development/overview.md) -- all four plugin types and their interfaces
- [CLI Reference](../user-guide/cli-reference.md) -- full command reference (use `zhi labels --help` for label-specific flags)
