# Lesson 6: Advanced Topics and Next Steps

This final lesson ties together what you have learned and introduces advanced features for production use: security, plugin development, the MCP integration for AI assistants, and the enterprise mirror.

## Prerequisites

- Completed [Lesson 1](lesson-01-getting-started.md) through [Lesson 5](lesson-05-metadata-labels.md)

## Recap

Over the course of these lessons you have learned:

1. **Getting Started** -- workspace initialization, reading and writing values, validation, export
2. **Export and Apply** -- template rendering, provisioning commands, named targets, drift detection
3. **Components** -- grouping configuration, toggling features, dependencies, mandatory components
4. **Workspaces and Marketplace** -- installing workspaces from OCI registries, browsing and managing plugins
5. **Metadata Labels** -- semantic annotations that control UI, store, transform, and config behavior

## Security Features

### Plugin Signature Verification

All official plugins are signed with keyless cosign via Sigstore. When you install a plugin, zhi verifies its signature by default:

```sh
# Verify without installing
zhi plugin verify oci://ghcr.io/mrwong99/zhi/zhi-store-vault:v1.10.2
```

You can enforce strict verification with a security policy in `~/.zhi/policy.yaml`:

```yaml
requireSignatures: true
allowedRegistries:
  - ghcr.io
trustedPublishers:
  - mrwong99
```

### Binary Integrity

At launch time, zhi compares each plugin binary's SHA-256 digest against the digest recorded during installation. A mismatch (indicating post-install tampering) is logged as an error.

### Version Pinning and Rollback

Pin a plugin to prevent automatic updates:

```sh
zhi plugin pin vault
zhi plugin unpin vault
```

Roll back to the previous version after an update:

```sh
zhi plugin rollback vault
```

## Building Your Own Plugins

zhi supports four plugin types, all communicating over gRPC:

| Type | Purpose |
|------|---------|
| Config | Manage and validate configuration values |
| Transform | Mutate configuration before display or after save |
| Store | Persist and retrieve configuration trees |
| UI | Provide interactive frontends |

### Scaffolding

Generate a complete plugin project:

```sh
zhi plugin new --name my-config --type config --author myorg
```

This creates a standalone Go project with implementation stubs, tests, a Makefile, CI/CD workflows, and a sample workspace.

### Publishing

Build for multiple platforms and publish:

```sh
zhi plugin publish --registry ghcr.io/myorg --sign
```

### Java Plugins

As shown in [Lesson 5](lesson-05-metadata-labels.md), plugins can be written in Java (or any language with gRPC support). The `zhi-config-javabean` example in the repository demonstrates Bean Validation integration and GraalVM native-image compilation.

## MCP Integration for AI Assistants

zhi includes an MCP (Model Context Protocol) server that lets AI assistants like Claude Desktop, Claude Code, and Cursor interact with your configuration:

```sh
zhi edit --ui mcp-stdio
```

This starts an MCP server over stdin/stdout. AI clients can then browse the tree, edit values, validate, export, and apply -- all through the standard MCP tool interface.

For network-based MCP access (e.g., from a remote AI client), use the `zhi-ui-mcp-sse` plugin:

```yaml
ui:
  provider: mcp-sse
  options:
    addr: "0.0.0.0:9090"
    token: "my-secret-token"
```

## Vault Credential Management

The `zhi-store-vault-manager` meta-plugin automates credential lifecycle management for deployed applications. It generates least-privilege Vault policies, creates AppRole roles, and injects ephemeral credentials -- all driven by metadata labels.

Bootstrap it with:

```sh
zhi vault-credentials bootstrap
```

And refresh credentials without a full export/apply cycle:

```sh
zhi vault-credentials refresh --app web-api --output env
```

## Enterprise Mirror

For air-gapped environments, the `zhi-mirror` server provides a local OCI registry that mirrors upstream plugins:

```sh
zhi-mirror serve --listen :5050 --upstream-registry ghcr.io
```

Configure clients to use it:

```yaml
# ~/.zhi/config.yaml
registries:
  mirror.internal:5050:
    username: user
    password: token
```

## Lock Files for Reproducible Builds

Pin all plugin dependencies to exact OCI digests:

```sh
zhi workspace lock
```

This generates a `zhi-plugins.lock` file. CI/CD pipelines and team members who run `zhi workspace setup` install identical plugin versions.

## File Rollback

zhi keeps a snapshot of exported files before each export or apply. If something goes wrong, roll back:

```sh
zhi rollback
```

This restores all exported files to their state before the last `zhi export` or `zhi apply`.

## Summary

zhi provides a security-first approach to configuration management with:

- An extensible plugin system supporting four plugin types in any gRPC language
- Metadata labels for declarative control over UI rendering, storage, and validation
- OCI-based sharing of both plugins and workspaces
- Template-based exports that drive any provisioning tool
- Enterprise features like Vault integration, signature verification, and air-gapped mirrors
- AI assistant integration via MCP

## Further Reading

### User Guide

- [Getting Started](../user-guide/getting-started.md)
- [Workspace Configuration](../user-guide/workspace-configuration.md)
- [CLI Reference](../user-guide/cli-reference.md)
- [Components](../user-guide/components.md)
- [Export and Templates](../user-guide/export-and-templates.md)
- [Apply](../user-guide/apply.md)
- [Plugin Discovery](../user-guide/plugin-discovery.md)
- [Sharing and Registries](../user-guide/sharing-and-registries.md)
- [Web UI](../user-guide/web-ui.md)
- [Vault Credentials](../user-guide/vault-credentials.md)
- [Enterprise Mirror](../user-guide/enterprise-mirror.md)

### Plugin Development

- [Plugin Development Overview](../plugin-development/overview.md)
- [Config Plugin API](../plugin-development/config-plugin.md)
- [Transform Plugin API](../plugin-development/transform-plugin.md)
- [Store Plugin API](../plugin-development/store-plugin.md)
- [UI Plugin API](../plugin-development/ui-plugin.md)
- [Meta-Plugin SDK](../plugin-development/meta-plugin.md)
- [Java Plugin Development](../plugin-development/java-plugin.md)
- [Plugin Scaffolding](../plugin-development/scaffolding.md)

### Design Documents

- [Metadata Labels API Design](../design/metadata-labels-api.md)
