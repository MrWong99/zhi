# Doctor

`zhi doctor` runs a battery of health checks against the current workspace and reports findings grouped by category. Think of it as `brew doctor` for zhi: one command that answers "is my workspace healthy?".

## Quick start

```bash
zhi doctor
```

Produces output similar to:

```
Running zhi doctor…

[OK]   Workspace — 4/4 checks passed (1 skipped)
  [OK] workspace present — loaded from /home/alice/projects/infra
  [OK] workspace parses — workspace.yaml parsed successfully
  [OK] workspace validates — workspace validates cleanly
  [OK] plugin directory — /home/alice/.zhi/plugins
  [SKIP] file store path — store provider is not file-backed

[WARN] Plugins — 12/13 checks passed (1 warning, 1 skipped)
  [OK] config/structuredfile — resolved
  [WARN] config/glyphoxa — no manifest or install metadata
         hint: plugin will still work but version-compat checks are skipped
  ...

Summary: 16/17 checks passed, 1 warning(s), 0 error(s), 2 skipped
```

`zhi doctor` exits 0 if no checks failed, 1 if any check reported an error. Warnings do not affect the exit code.

## Check categories

| Category | What it verifies |
|----------|------------------|
| `workspace` | `zhi.yaml` exists, parses, validates; plugin directories are accessible; file-backed store paths exist |
| `plugins` | Every referenced provider resolves; installed plugins have readable metadata; version compatibility against the current zhi; plugin processes launch and respond to a cheap RPC |
| `store` | Store plugin responds to `Capabilities()` and `ListTrees()`; Vault-specific probes when the store provider is `vault` (seal status, token validity and TTL, mount accessibility) |
| `config` | `engine.Validate()` runs without blocking results; no `core.deprecated` fields remain in use; literal `${VAR}` patterns are flagged (zhi does not yet expand env vars) |
| `updates` | Installed plugin versions are compared against the marketplace — opt-in, requires network |

Filter with `--check`. The reserved name `all` expands to every category, including those that need the network:

```bash
zhi doctor --check workspace
zhi doctor --check plugins --check store
zhi doctor --check updates                 # marketplace lookup only
zhi doctor --check all                     # everything, including updates
```

## JSON output

```bash
zhi doctor --json | jq .summary
```

```json
{
  "ok": 16,
  "warnings": 1,
  "errors": 0,
  "skipped": 2,
  "total": 17,
  "exit_code": 0
}
```

The full payload contains a `groups` array keyed by category, each with a `results` array of `{check_id, name, status, message, detail, hint, fixable}` objects.

## Options

| Flag | Purpose |
|------|---------|
| `--check <category>` | Run only the named category. Repeatable. `all` expands to every category. |
| `--json` | Machine-readable output. Suppresses colors and headings. |
| `--quiet` | Skip OK and Skipped results in text output; useful when piping to review summaries. |
| `--deep` | Enable slow checks — currently plugin signature verification against `zhi-plugins.lock`. |
| `--timeout <duration>` | Per-check wall-clock deadline (default 10s). |
| `--no-color` | Disable ANSI output. Also honours `NO_COLOR`. |
| `--fix` | Apply auto-fixable remediations. The v1 fixer set is empty — this flag is reserved for future releases. |

## Vault checks

When the workspace declares `store.provider: vault`, the `store` category runs five additional probes using the connection settings from `zhi.yaml` and the standard `VAULT_*` environment variables:

- `store.vault.reachable` — hits `sys/seal-status`.
- `store.vault.unsealed` — confirms `initialized=true, sealed=false`.
- `store.vault.token_valid` — calls `auth/token/lookup-self`.
- `store.vault.token_ttl` — warns when the token has under one hour remaining.
- `store.vault.mount_accessible` — lists the configured mount/prefix to verify `list` permission.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed or produced only warnings |
| `1` | At least one check reported an error |

## What doctor does not do

- It does not modify the workspace or any plugin state. `--fix` is a placeholder; no fixers are registered in this release.
- It does not expand environment variables; zhi does not yet support `${VAR}` expansion. The `config.env_var_expansion` check flags literal patterns so you know they are stored verbatim.
- It does not probe UI plugins. UI plugins typically need a TTY and would block a non-interactive health check.
