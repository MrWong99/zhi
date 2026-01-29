# Step 8: CI/CD and Release

## Overview

Set up the CI/CD pipeline and release automation to build, test, and distribute the zhi binary across platforms. This step makes zhi installable and ensures ongoing quality through automated checks.

## Relevant Existing Files

- `Makefile` — existing build commands (`make build`, `make test`, `make check`, `make lint`, etc.)
- `.github/ISSUE_TEMPLATE/` — existing GitHub issue templates
- `go.mod` — Go 1.24+, module `github.com/MrWong99/zhi`
- `CONTRIBUTING.md` — contribution guidelines
- `LICENSE` — MIT license

## Implementation Plan

### 8.1 GitHub Actions CI Pipeline (`.github/workflows/ci.yml`)

Automated checks on every push and pull request.

**Jobs:**

#### `lint` job:
- Go version: 1.24.x
- Steps:
  1. Checkout code
  2. Set up Go
  3. Run `make fmt` and check for uncommitted changes (formatting check)
  4. Run `make lint` (golangci-lint)
  5. Run `go vet ./...`

#### `test` job:
- Go version: 1.24.x
- Matrix: `ubuntu-latest`, `macos-latest` (Linux + macOS coverage)
- Steps:
  1. Checkout code
  2. Set up Go
  3. Run `make test` (race detection enabled)
  4. Upload coverage report (optional: codecov integration)

#### `build` job:
- Go version: 1.24.x
- Matrix: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- Steps:
  1. Checkout code
  2. Set up Go
  3. Run `make build` with `GOOS` and `GOARCH` set
  4. Verify binary runs: `./bin/zhi --help`

#### `proto-check` job:
- Steps:
  1. Checkout code
  2. Install protoc, protoc-gen-go, protoc-gen-go-grpc
  3. Run `make proto-check` (verify generated code is up-to-date)

#### `integration` job (depends on `build`):
- Steps:
  1. Checkout code
  2. Set up Go
  3. Run `make build-all` (build main binary + example plugins)
  4. Run integration tests from `test/`

### 8.2 GoReleaser Configuration (`.goreleaser.yaml`)

Automate cross-compilation and release packaging.

**Configuration:**

```yaml
version: 2

before:
  hooks:
    - go mod tidy
    - make proto-check

builds:
  - id: zhi
    main: ./cmd/zhi/
    binary: zhi
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - id: default
    formats:
      - tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE
      - README.md

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
      - "^chore:"

release:
  github:
    owner: MrWong99
    name: zhi
  draft: true
  prerelease: auto
```

### 8.3 Release GitHub Action (`.github/workflows/release.yml`)

Triggered on tag push (`v*`).

**Steps:**

1. Checkout code with full history (`fetch-depth: 0`)
2. Set up Go 1.24.x
3. Run GoReleaser with `goreleaser release --clean`
4. Upload artifacts to GitHub Release

### 8.4 Version Information (`cmd/zhi/version.go`)

Embed build version info into the binary.

**Components:**

- `var version, commit, date string` — set by ldflags at build time
- `zhi version` subcommand — print version, commit hash, build date
- `zhi --version` flag — print version string

**Output:**
```
$ zhi version
zhi v0.1.0 (commit: abc1234, built: 2026-01-29)
```

### 8.5 Makefile Updates

Extend the Makefile for release workflows:

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
    go build -ldflags "$(LDFLAGS)" -o bin/zhi ./cmd/zhi/

snapshot:
    goreleaser release --snapshot --clean

release-dry-run:
    goreleaser release --skip=publish --clean
```

### 8.6 Dependency Management

- Pin all direct dependencies to exact versions in `go.mod`
- Use `go mod tidy` in CI to verify no missing/extra dependencies
- Use `go mod verify` to check module checksums
- Consider Dependabot or Renovate for automated dependency updates

### 8.7 Branch Protection

Recommend (document, don't enforce via code):

- Require CI pass before merge to `main`
- Require at least one review for PRs
- No force-push to `main`
- Squash merge by default

### 8.8 Release Process Documentation

Add to CONTRIBUTING.md or a new section in README.md:

**Release checklist:**

1. Ensure `main` is green (all CI checks pass)
2. Update CHANGELOG.md (if maintained)
3. Create and push a version tag: `git tag v0.1.0 && git push origin v0.1.0`
4. GoReleaser automatically builds and creates a draft GitHub Release
5. Review the draft release, edit release notes if needed
6. Publish the release

**Versioning policy:**

- Follow Semantic Versioning (SemVer)
- Pre-1.0: breaking changes may occur in minor versions
- Plugin protocol version is tracked separately (`ProtocolVersion` in handshake)

### 8.9 Install Instructions

Add to README.md:

**From release:**
```bash
# Linux amd64
curl -sSL https://github.com/MrWong99/zhi/releases/latest/download/zhi_linux_amd64.tar.gz | tar xz
sudo mv zhi /usr/local/bin/

# macOS arm64 (Apple Silicon)
curl -sSL https://github.com/MrWong99/zhi/releases/latest/download/zhi_darwin_arm64.tar.gz | tar xz
sudo mv zhi /usr/local/bin/
```

**From source:**
```bash
go install github.com/MrWong99/zhi/cmd/zhi@latest
```

### 8.10 Tests

- Verify `goreleaser check` passes (validates `.goreleaser.yaml`)
- Verify `goreleaser release --snapshot --clean` builds all targets locally
- Verify `zhi version` prints correct version info when built with ldflags
- Verify CI pipeline runs successfully on a test branch

## Verification Criteria

1. CI pipeline runs on every push and PR: lint, test, build, proto-check
2. Tests run on both Linux and macOS
3. Build produces binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
4. `goreleaser check` validates the release configuration
5. `goreleaser release --snapshot --clean` builds all targets locally
6. `zhi version` prints version, commit, and build date
7. Tag push triggers the release workflow
8. Release creates a draft GitHub Release with binary archives and checksums
9. Archives include LICENSE and README.md alongside the binary
10. `make check` passes (includes all lint, vet, fmt, test checks)
11. Install instructions in README.md work for a fresh download
