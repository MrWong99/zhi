# Security Policy

Security is a core principle of **zhi** -- it's right there in the name
(智 -- *wisdom*). We take vulnerability reports seriously and appreciate
the community's help in keeping the project safe.

## Supported Versions

| Version              | Supported |
|----------------------|-----------|
| `main` (development) | Yes       |

As the project matures and tagged releases are published, this table
will be updated to reflect which versions receive security fixes.

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Instead, report them privately using one of the following methods:

1. **GitHub Security Advisories** (preferred) --
   [Open a private security advisory](https://github.com/MrWong99/zhi/security/advisories/new)
   directly on this repository.
2. **Email** -- reach out to the maintainer at the email address listed
   on their [GitHub profile](https://github.com/MrWong99).

### What to include

- A clear description of the vulnerability and its potential impact.
- Steps to reproduce the issue or a minimal proof of concept.
- The version(s) or commit(s) affected.
- Any suggested fix, if you have one.

### What to expect

- **Acknowledgement** within 72 hours of your report.
- We will work with you to understand and validate the issue.
- A fix will be developed privately and disclosed responsibly.
- You will be credited in the release notes (unless you prefer to
  remain anonymous).

## Security Design

zhi is built with security in mind from the ground up:

- **Encrypted configuration at rest** -- secrets never touch disk in
  plain text.
- **Plugin isolation** -- plugins run as separate processes communicating
  over gRPC, limiting the blast radius of any single component.
- **Validation before deployment** -- configuration is validated at
  multiple stages to prevent misconfigurations from reaching production.

## Scope

The following are in scope for security reports:

- The `zhi` CLI and core libraries (`pkg/`, `internal/`, `cmd/`)
- The plugin framework and gRPC transport (`pkg/zhiplugin/`)
- Built-in providers (`pkg/providers/`)
- Protocol Buffer definitions and generated code (`api/proto/`)

The following are **out of scope**:

- Example plugins under `examples/` (these are intentionally simplified
  for learning purposes -- even a Pokedex has known weaknesses)
- Third-party dependencies (please report those to the upstream project,
  but do let us know if a dependency vulnerability affects zhi)

---

*A wise Trainer secures their Pokedex before venturing into tall grass.
Thank you for helping us keep zhi safe.*
