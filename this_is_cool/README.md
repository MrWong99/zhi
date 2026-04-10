# This Is Cool: An Interactive Introduction to zhi

<audio controls src="audio/00-intro.mp3">
  Your browser does not support the audio element.
</audio>

Welcome! This is a hands-on, notebook-style tutorial that walks you through **zhi** --
a security-first platform for configuration management and provisioning.

By the end you'll have:

- A running **HashiCorp Vault** instance (deployed and bootstrapped by zhi)
- A **zhi marketplace** and **mirror** serving real plugins
- Hands-on experience with zhi's TUI, Web UI, and AI-assisted editing
- Your own custom workspace backed by Vault

## Prerequisites

| Tool | Why |
|------|-----|
| **Docker** (with Compose plugin) | We run Vault, mirror, and marketplace as containers |
| **curl / jq** | For downloading binaries and inspecting APIs |
| **A terminal** | All lessons are designed for the command line |

Optional but recommended:

| Tool | Why |
|------|-----|
| **Claude Code** | Lesson 03 showcases AI-assisted config editing via MCP |
| **A web browser** | Lesson 03 showcases the Web UI |

## How to Use These Notebooks

Each lesson is a Markdown file with executable code blocks. You have two ways to run them:

### Option A: VS Code with Runme Extension (recommended)

1. Install the [Runme extension](https://marketplace.visualstudio.com/items?itemName=stateful.runme) in VS Code
2. Open any `.md` file in this directory
3. Code blocks become executable cells -- click the play button next to each one

This gives you the best notebook experience: inline output, easy re-runs, and
a clear visual separation between prose and code.

### Option B: Runme CLI with `runme open`

If you prefer the terminal, install the Runme CLI and use `runme open` to launch
a browser-based notebook UI:

```sh {"name": "install-runme", "interactive": true}
# Install Runme CLI (pick one)
# macOS / Linux (Homebrew):
brew install runme

# Or via npm:
# npm install -g runme

# Or download from https://github.com/stateful/runme/releases
```

```sh {"name": "open-lesson", "excludeFromRunAll": true, "interactive": true}
# Open a lesson in the browser-based notebook UI
runme open --filename 01-what-is-zhi.md
```

You can also run individual named cells directly from the CLI:

```sh {"name": "run-cell-example", "excludeFromRunAll": true, "interactive": true}
# Run a specific named cell
runme run --filename 01-what-is-zhi.md verify-zhi
```

## Lessons

| # | File | What You'll Learn |
|---|------|-------------------|
| 1 | [What is zhi?](01-what-is-zhi.md) | Install zhi, explore the CLI, understand the plugin architecture |
| 2 | [Your First Workspace](02-your-first-workspace.md) | Create a workspace, browse configs, edit values, validate, export, apply |
| 3 | [Deploy Vault](03-vault-setup.md) | Deploy Vault with Docker, experience TUI / Web UI / MCP editing |
| 4 | [zhi Infrastructure](04-zhi-infrastructure.md) | Install workspaces from OCI registries, use the Web UI marketplace |
| 5 | [Metadata Labels](05-plugins.md) | Discover labels, control UI/store behavior, Java config plugin |
| 6 | [Bring It Together](06-bring-it-together.md) | Build a custom workspace backed by your Vault with labels |

Each lesson builds on the previous one. Start with lesson 1 and work your way through.

## Cleanup

When you're done exploring, tear everything down:

```sh {"name": "cleanup-all", "interactive": true}
# Stop and remove all containers from the tutorial
docker compose -f ../deploy/vault/docker-compose.yml -p vault down -v 2>/dev/null
docker compose -f ../deploy/zhi/docker-compose.yml -p zhi down -v 2>/dev/null
echo "All cleaned up!"
```

## Troubleshooting

- **Docker not running?** Start Docker Desktop or `sudo systemctl start docker`
- **Port conflict?** Check if 8200 (Vault), 8080 (mirror), or 8090 (marketplace) are in use
- **Runme not found?** Make sure it's in your PATH, or use VS Code with the Runme extension
