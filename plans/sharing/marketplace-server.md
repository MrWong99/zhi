# Marketplace Server

The central marketplace is a lightweight discovery and metadata service. It does **not** store artifacts — those live in OCI registries. The marketplace indexes what exists, provides search, and hosts community features (ratings, verification badges, download counts).

## Design Philosophy

The marketplace is a **thin metadata layer** over OCI registries, similar to how Artifact Hub indexes Helm charts without storing them. This keeps infrastructure costs low and lets publishers use whatever OCI registry they prefer.

## API Design

Base URL: `https://marketplace.zhi.dev/api/v1`

### Discovery Endpoint

```
GET /.well-known/zhi-marketplace.json
{
  "api": "https://marketplace.zhi.dev/api/v1",
  "version": "1",
  "registries": ["ghcr.io", "docker.io"],
  "signingKey": "https://marketplace.zhi.dev/.well-known/zhi-signing-key.pub"
}
```

This allows any domain to act as a marketplace by hosting this well-known file.

### Search

```
GET /api/v1/search?q=ansible&type=config&sort=downloads&page=1&per_page=20

{
  "total": 3,
  "results": [
    {
      "name": "ansible-config",
      "type": "config",
      "description": "Configuration provider backed by Ansible inventory",
      "author": "zhi-project",
      "verified": true,
      "latestVersion": "1.2.0",
      "ociRef": "oci://ghcr.io/zhi-project/zhi-config-ansible",
      "downloads": 12450,
      "rating": 4.7,
      "ratingCount": 89,
      "keywords": ["ansible", "inventory", "config"],
      "updatedAt": "2026-01-15T10:30:00Z",
      "platforms": ["linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"]
    }
  ]
}
```

### Plugin/Workspace Detail

```
GET /api/v1/plugins/zhi-project/ansible-config

{
  "name": "ansible-config",
  "type": "config",
  "author": "zhi-project",
  "verified": true,
  "description": "Configuration provider backed by Ansible inventory",
  "longDescription": "...",  # from README layer
  "license": "Apache-2.0",
  "homepage": "https://github.com/zhi-project/zhi-config-ansible",
  "repository": "https://github.com/zhi-project/zhi-config-ansible",
  "ociRef": "oci://ghcr.io/zhi-project/zhi-config-ansible",
  "versions": [
    {
      "version": "1.2.0",
      "digest": "sha256:abc123...",
      "zhiProtocolVersion": "1",
      "minZhiVersion": "0.5.0",
      "platforms": ["linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"],
      "runtime": null,
      "publishedAt": "2026-01-15T10:30:00Z",
      "signed": true,
      "downloads": 3200
    },
    {
      "version": "1.1.0",
      "digest": "sha256:def456...",
      "publishedAt": "2025-11-01T08:00:00Z",
      "downloads": 9250
    }
  ],
  "rating": {
    "average": 4.7,
    "count": 89,
    "distribution": { "5": 52, "4": 28, "3": 7, "2": 1, "1": 1 }
  },
  "statistics": {
    "totalDownloads": 12450,
    "monthlyDownloads": 1890,
    "dependents": 5
  }
}
```

### Version Listing

```
GET /api/v1/plugins/zhi-project/ansible-config/versions

{
  "versions": [
    {
      "version": "1.2.0",
      "platforms": [
        { "os": "linux", "arch": "amd64" },
        { "os": "linux", "arch": "arm64" },
        { "os": "darwin", "arch": "amd64" },
        { "os": "darwin", "arch": "arm64" }
      ],
      "digest": "sha256:abc123...",
      "signed": true,
      "publishedAt": "2026-01-15T10:30:00Z"
    }
  ]
}
```

### Download Resolution

```
GET /api/v1/plugins/zhi-project/ansible-config/1.2.0/resolve?os=linux&arch=amd64

{
  "ociRef": "oci://ghcr.io/zhi-project/zhi-config-ansible:v1.2.0",
  "digest": "sha256:abc123...",
  "signed": true,
  "signingIdentity": "release@zhi.dev",
  "checksums": {
    "binary": "sha256:789xyz..."
  }
}
```

### Ratings

```
# Submit a rating (authenticated)
POST /api/v1/plugins/zhi-project/ansible-config/ratings
Authorization: Bearer <token>
{
  "score": 5,
  "comment": "Works perfectly with our Ansible inventory structure."
}

# List ratings
GET /api/v1/plugins/zhi-project/ansible-config/ratings?page=1

{
  "ratings": [
    {
      "user": "alice",
      "score": 5,
      "comment": "Works perfectly with our Ansible inventory structure.",
      "createdAt": "2026-01-20T14:00:00Z",
      "helpful": 12
    }
  ]
}
```

### Publishing (Registration)

```
# Register a new plugin with the marketplace
POST /api/v1/plugins
Authorization: Bearer <token>
{
  "name": "my-config-plugin",
  "type": "config",
  "ociRef": "oci://ghcr.io/myorg/zhi-config-myplugin",
  "description": "My custom config provider",
  "homepage": "https://github.com/myorg/zhi-config-myplugin",
  "keywords": ["custom", "config"]
}

# Notify marketplace of a new version (called after OCI push)
POST /api/v1/plugins/myorg/my-config-plugin/versions
Authorization: Bearer <token>
{
  "version": "1.0.0",
  "digest": "sha256:abc123...",
  "platforms": ["linux/amd64", "darwin/arm64"],
  "zhiProtocolVersion": "1",
  "changelog": "Initial release"
}
```

## Database Schema

```sql
-- Publishers
CREATE TABLE publishers (
    id          UUID PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,      -- e.g., "zhi-project", "myorg"
    email       TEXT NOT NULL,
    verified    BOOLEAN DEFAULT FALSE,     -- verified publisher badge
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Plugins and workspaces
CREATE TABLE artifacts (
    id          UUID PRIMARY KEY,
    publisher_id UUID REFERENCES publishers(id),
    name        TEXT NOT NULL,             -- e.g., "ansible-config"
    type        TEXT NOT NULL,             -- "config", "transform", "store", "ui", "workspace"
    oci_ref     TEXT NOT NULL,             -- OCI reference (registry/repo)
    description TEXT,
    long_description TEXT,                 -- from README
    license     TEXT,
    homepage    TEXT,
    repository  TEXT,
    keywords    TEXT[],
    verified    BOOLEAN DEFAULT FALSE,     -- inherits from publisher or manual review
    deprecated  BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(publisher_id, name)
);

-- Versions
CREATE TABLE versions (
    id          UUID PRIMARY KEY,
    artifact_id UUID REFERENCES artifacts(id),
    version     TEXT NOT NULL,             -- semver
    digest      TEXT NOT NULL,             -- OCI manifest digest
    platforms   JSONB NOT NULL,            -- [{"os":"linux","arch":"amd64"}, ...]
    zhi_protocol_version TEXT,
    min_zhi_version TEXT,
    runtime     JSONB,                     -- runtime requirements
    dependencies JSONB,                    -- workspace plugin dependencies
    changelog   TEXT,
    signed      BOOLEAN DEFAULT FALSE,
    signing_identity TEXT,
    published_at TIMESTAMPTZ DEFAULT NOW(),
    yanked      BOOLEAN DEFAULT FALSE,     -- soft-delete without removing
    UNIQUE(artifact_id, version)
);

-- Download counts (append-only for analytics)
CREATE TABLE downloads (
    id          UUID PRIMARY KEY,
    version_id  UUID REFERENCES versions(id),
    platform    TEXT,                       -- "linux/amd64"
    downloaded_at TIMESTAMPTZ DEFAULT NOW()
);

-- Ratings
CREATE TABLE ratings (
    id          UUID PRIMARY KEY,
    artifact_id UUID REFERENCES artifacts(id),
    user_id     UUID REFERENCES publishers(id),  -- reuse publisher accounts
    score       INTEGER CHECK (score BETWEEN 1 AND 5),
    comment     TEXT,
    helpful     INTEGER DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(artifact_id, user_id)              -- one rating per user per artifact
);

-- Indexes
CREATE INDEX idx_artifacts_type ON artifacts(type);
CREATE INDEX idx_artifacts_keywords ON artifacts USING GIN(keywords);
CREATE INDEX idx_versions_artifact ON versions(artifact_id, published_at DESC);
CREATE INDEX idx_downloads_version ON downloads(version_id);
CREATE INDEX idx_ratings_artifact ON ratings(artifact_id);
```

## Server Implementation

### Technology Stack

- **Language**: Go (consistent with zhi codebase)
- **Framework**: Standard library `net/http` with a router (chi or gorilla/mux)
- **Database**: PostgreSQL (or SQLite for self-hosted/small instances)
- **Authentication**: GitHub OAuth for publisher accounts, API keys for CLI
- **Search**: PostgreSQL full-text search (sufficient for the scale; upgrade to Elasticsearch if needed)
- **Hosting**: Could be deployed as a single binary with an embedded SQLite database for small deployments, or as a containerized service with PostgreSQL for the central instance

### Self-Hosted Marketplace

Organizations can run their own marketplace server pointing to their internal OCI registries:

```yaml
# ~/.zhi/config.yaml
marketplace:
  url: https://zhi-marketplace.internal.company.com
  apiKey: zhk_abc123...
```

The marketplace server is a single Go binary:

```bash
zhi-marketplace serve \
  --db postgres://... \
  --listen :8080 \
  --oci-registries harbor.internal:5000
```

### Metadata Sync

The marketplace periodically validates that OCI references still exist and updates metadata (README content, platform list) by inspecting the OCI manifests. This prevents stale entries from accumulating.

```
Sync loop (hourly):
  For each artifact:
    → HEAD the OCI manifest for :latest tag
    → Update platforms, digest, signed status
    → Pull README layer if changed
    → Mark as unavailable if manifest is gone
```

## Categories and Curation

The marketplace organizes artifacts into categories for browsing:

### Plugin Categories
- **Config Providers**: File-based, database-backed, cloud-native, inventory
- **Transform Providers**: Security, compliance, formatting, templating
- **Store Providers**: Vault, cloud KMS, file-based, database
- **UI Plugins**: TUI, Web, API, IDE integrations

### Workspace Categories
- **Infrastructure**: Kubernetes, Terraform, Ansible, cloud providers
- **Application**: Database configs, web servers, microservices
- **Security**: TLS/certificate management, secrets rotation, compliance
- **Development**: Local dev environments, CI/CD pipelines

### Featured and Curated Lists
- **Official**: Maintained by the zhi project team
- **Community Picks**: Highly rated community plugins
- **New & Notable**: Recently published with good early adoption
- **Getting Started**: Recommended for new users
