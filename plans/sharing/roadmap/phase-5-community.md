# Phase 5: Community — Ratings & Verification

**Goal**: Trust signals help users choose reliable plugins.

**Prerequisites**: [Phase 4 (Discovery)](phase-4-discovery.md) — the marketplace server must exist for ratings and verification data to be stored and displayed.

## Overview

This phase adds the community trust layer: star ratings with text reviews, a verified publisher program, download statistics, and vulnerability advisories. These signals surface in `zhi plugin search`, `zhi plugin info`, and later in UI plugins ([Phase 6](phase-6-ui-integration.md)).

The full design for ratings and verification is in the [Ratings & Verification](../ratings-and-verification.md) plan. Security advisories integrate with the update mechanism designed in the [Update Mechanism](../update-mechanism.md) plan.

## Deliverables

### 1. Rating System

**Submit ratings** via CLI or marketplace API:
- `zhi plugin rate <name>` — Interactive rating (1-5 stars + optional comment)
- `POST /api/v1/plugins/{publisher}/{name}/ratings` — API endpoint
- Requires authenticated marketplace account (GitHub OAuth or API key)
- One rating per user per artifact (can be updated)

**Display ratings**:
- Bayesian weighted average to prevent small sample distortion (see [Ratings & Verification](../ratings-and-verification.md) for formula)
- Star distribution histogram in `zhi plugin info`
- Average score in `zhi plugin search` results
- Helpfulness voting on reviews

**Marketplace API additions** (extending [Marketplace Server](../marketplace-server.md)):
- `GET /api/v1/plugins/{publisher}/{name}/ratings` — List ratings with pagination
- `POST /api/v1/plugins/{publisher}/{name}/ratings` — Submit rating
- `POST /api/v1/plugins/{publisher}/{name}/ratings/{id}/helpful` — Mark as helpful

### 2. Verified Publisher Program

Organizational verification as described in [Ratings & Verification](../ratings-and-verification.md):

**Verification tiers**:
| Tier | Badge | Criteria |
|---|---|---|
| 0: Unverified | (none) | Any authenticated publisher |
| 1: Authenticated | Identity shown | GitHub/GitLab OAuth |
| 2: Verified | ✓ | Public source, OSI license, signed releases, CI tests, active maintenance |
| 3: Official | ★ | Maintained by zhi project team |

**Verification workflow**:
1. Publisher applies via `zhi plugin verify-request` or marketplace UI
2. Automated checks: license detection, security scan (`gosec`), signing verification, CI pipeline check
3. Manual review by zhi maintainer (code quality, documentation)
4. Badge applied; annual re-verification

**Marketplace API additions**:
- `POST /api/v1/publishers/{name}/verify-request` — Apply for verification
- `GET /api/v1/publishers/{name}` — Publisher profile with verification status

### 3. Download Statistics

Track and display download counts:

- Record downloads in the marketplace database (see `downloads` table in [Marketplace Server](../marketplace-server.md) schema)
- Anonymized: only platform and timestamp, no user identity
- Display in search results, plugin info, and marketplace website
- Trend indicators: total downloads, monthly downloads, month-over-month change

### 4. Vulnerability Advisories

A security advisory system for disclosed vulnerabilities:

**Advisory database** in marketplace:
- `POST /api/v1/advisories` — Publish advisory (maintainers and verified publishers)
- `GET /api/v1/advisories` — List advisories, filterable by plugin and severity
- Advisory fields: affected plugin, affected versions, severity, description, fixed version, CVE reference

**Client integration**:
- `zhi plugin update --check` cross-references installed plugins against advisories
- Affected plugins shown with severity level and recommended action
- Feeds into [Phase 8 (Updates)](phase-8-updates.md) for automated remediation

## Key Files to Modify

| File | Change |
|---|---|
| `cmd/zhi-marketplace/` | Add rating, verification, advisory, and statistics endpoints |
| `internal/cli/plugin_search.go` | Display ratings and verified badges in search results |
| `internal/cli/plugin_info.go` | Display rating distribution, verification status, download stats |

## New Files

```
internal/cli/plugin_rate.go           # zhi plugin rate command
cmd/zhi-marketplace/server/ratings.go # Rating endpoints and logic
cmd/zhi-marketplace/server/verify.go  # Verification workflow
cmd/zhi-marketplace/server/advisory.go # Vulnerability advisory endpoints
cmd/zhi-marketplace/server/stats.go   # Download statistics tracking
```

## Anti-Abuse Measures

As described in [Ratings & Verification](../ratings-and-verification.md):

- New accounts: 24-hour cooldown before rating
- Rate limiting: 10 ratings/hour per user
- Self-rating detection (publisher rating own plugin)
- Reviews with links flagged for moderation
- Bulk identical rating detection
- Publisher dispute resolution process

## Exit Criteria

- Users can rate plugins (1-5 stars + review) via CLI and API
- Ratings display in `zhi plugin search` and `zhi plugin info` with Bayesian averages
- Verified publishers show ✓ in all CLI output
- Download statistics are tracked and displayed
- `zhi plugin update --check` warns about security advisories affecting installed plugins
- Abuse prevention measures are active

## Design References

- [Ratings & Verification](../ratings-and-verification.md) — Full rating model, Bayesian formula, verification tiers and criteria, quality signals, community features, abuse prevention
- [Marketplace Server](../marketplace-server.md) — Database schema for ratings and downloads tables, API endpoints
- [Security & Trust](../security.md) — Verification ties into the broader security model
- [Update Mechanism](../update-mechanism.md) — Advisory integration with update checks
