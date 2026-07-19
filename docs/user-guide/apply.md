# Apply

The apply system runs a user-configured external command after optionally exporting configuration. This is how zhi triggers provisioning tools (Docker Compose, kubectl, Ansible, etc.) without embedding their SDKs.

## Configuration

Add an `apply` section to `zhi.yaml`:

```yaml
apply:
  command: "docker compose up -d"
  workdir: "."
  pre-export: true
  env:
    COMPOSE_PROJECT_NAME: "myapp"
  timeout: 300
```

| Field | Default | Description |
|-------|---------|-------------|
| `command` | (required) | Shell command to execute |
| `workdir` | `.` | Working directory, relative to workspace root |
| `pre-export` | `false` | Run all configured exports before the command |
| `env` | `{}` | Extra environment variables for the subprocess |
| `timeout` | `0` | Timeout in seconds (0 = no timeout) |

### Named Targets

Define multiple apply targets for different operations:

```yaml
apply:
  targets:
    default:
      command: "docker compose up -d"
      pre-export: true
    destroy:
      command: "docker compose down -v"
    restart:
      command: "docker compose restart"
```

Run a specific target:

```sh
zhi apply           # runs "default"
zhi apply destroy   # runs "destroy"
```

## Usage

```sh
zhi apply                     # Run the default apply target
zhi apply destroy             # Run a named target
zhi apply --dry-run           # Print command without executing
zhi apply --no-export         # Skip pre-export step
zhi apply --timeout 60        # Override timeout
zhi apply --env KEY=VALUE     # Add environment variable
```

**Example output:**

```
Exporting: docker-compose.override.yml ... done
Exporting: .env ... done
Running: docker compose up -d
[+] Running 3/3
 + Network myapp_default  Created
 + Container myapp-db-1   Started
 + Container myapp-app-1  Started

Apply completed (exit code 0)
```

## Component Awareness

When `pre-export: true` is configured, the exported files are filtered to enabled components only. Additionally, the subprocess environment includes:

| Variable | Description |
|----------|-------------|
| `ZHI_WORKSPACE` | Absolute path to the workspace root |
| `ZHI_ENABLED_COMPONENTS` | Comma-separated list of enabled component names |
| `ZHI_DISABLED_COMPONENTS` | Comma-separated list of disabled component names |

## Streaming Output

Apply streams stdout and stderr in real-time. In the CLI, output goes directly to your terminal. In the TUI, output appears in a scrollable viewport.

## Cancellation

Pressing `Ctrl+C` during apply sends `SIGTERM` to the subprocess. If it doesn't exit within a grace period, `SIGKILL` is sent.

## In the TUI

Press `a` in the TUI to open the apply view. Key bindings:

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll output |
| `G` | Jump to bottom |
| `g` | Jump to top |
| `Ctrl+C` | Cancel running command |
| `Esc` | Return to tree view (after command completes) |
