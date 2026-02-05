# Ratings & Verification

Community trust signals that help users choose reliable plugins and workspaces.

## Rating System

### Overview

Users can rate plugins and workspaces on a 1-5 star scale with an optional text review. Ratings are tied to authenticated marketplace accounts (GitHub OAuth) to prevent spam.

### Rating Model

```
Score:    1-5 stars (integer)
Comment:  Optional text review (max 2000 characters)
Per-user: One rating per artifact per user (can be updated)
Display:  Weighted average, shown to 1 decimal place
```

### Weighted Average

To avoid a single 1-star rating tanking a new plugin, ratings use a Bayesian average that blends the plugin's ratings with a global prior:

```
weighted_avg = (C * m + sum_of_ratings) / (C + num_ratings)

Where:
  C = confidence threshold (e.g., 10 ratings)
  m = global mean rating across all plugins (~4.0)
```

This means a new plugin with one 5-star rating shows ~4.1 instead of 5.0, and a new plugin with one 1-star rating shows ~3.7 instead of 1.0. As more ratings accumulate, the weighted average converges to the true average.

### Review Guidelines

Reviews are moderated for:
- Spam and promotional content
- Abusive or hateful language
- Reviews that are clearly about a different plugin

Reviews are **not** moderated for:
- Negative opinions (legitimate criticism is welcome)
- Feature requests expressed as feedback
- Comparisons with other plugins

### Helpfulness Voting

Users can mark reviews as "helpful", surfacing the most useful reviews:

```
GET /api/v1/plugins/zhi-project/ansible-config/ratings?sort=helpful

[
  {
    "user": "alice",
    "score": 5,
    "comment": "Seamlessly replaced our custom script. The YAML inventory support is excellent.",
    "helpful": 23,
    "createdAt": "2026-01-20T14:00:00Z"
  },
  {
    "user": "bob",
    "score": 3,
    "comment": "Works for basic cases but lacks support for dynamic inventory scripts.",
    "helpful": 15,
    "createdAt": "2026-01-25T09:00:00Z"
  }
]
```

### Rating Aggregation Display

```
★★★★★  4.7  (89 ratings)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━
★★★★★  ████████████████████  52  (58%)
★★★★   ██████████            28  (31%)
★★★    ██                     7  ( 8%)
★★                             1  ( 1%)
★                              1  ( 1%)
```

## Verification System

### Verification Tiers

```
Tier 0: Unverified
  → Anyone can publish. No identity verification.
  → Displayed without any badge.
  → Name shown as-is.

Tier 1: Authenticated Publisher
  → Publisher authenticated via GitHub/GitLab OAuth.
  → Identity is their GitHub/GitLab username or org.
  → Displayed with a small identity indicator.

Tier 2: Verified Publisher ✓
  → Publisher has been verified by the zhi project team.
  → Requirements:
    - Active GitHub/GitLab organization or well-known individual
    - Plugin source code is publicly available
    - Passes automated security scanning
    - Has a responsible disclosure process
  → Displayed with ✓ badge in search results, info, and UI.

Tier 3: Official ★
  → Maintained by the zhi project itself.
  → Plugins in the zhi-project namespace.
  → Displayed with ★ badge.
  → Highest trust level — these are the built-in providers published
    as external plugins for standalone use.
```

### Verification Criteria

To achieve Verified Publisher (Tier 2) status:

| Criterion | Requirement |
|---|---|
| **Identity** | GitHub/GitLab org with at least 6 months of history |
| **Source code** | Public repository with the plugin source |
| **License** | OSI-approved open-source license |
| **Security** | No known CVEs in latest version; passes `gosec` or equivalent |
| **Documentation** | README with usage instructions and example configuration |
| **Maintenance** | At least one release in the past 12 months |
| **Signing** | All releases signed with cosign |
| **Tests** | CI pipeline with automated tests |

### Verification Process

1. Publisher applies via marketplace UI or CLI (`zhi plugin verify-request`)
2. Automated checks run (license, security scan, signing verification)
3. Manual review by zhi maintainer (code quality, documentation)
4. If approved: ✓ badge is added; publisher notified
5. Annual re-verification: automated checks re-run; badge revoked if criteria no longer met

### Verification Revocation

Verification can be revoked if:
- Plugin is found to contain malicious code
- Publisher account is compromised and not remediated
- Plugin hasn't been updated for 18+ months (with warning at 12 months)
- Publisher requests removal

## Quality Signals

Beyond ratings and verification, the marketplace surfaces additional quality signals:

### Download Trends

```
Downloads:  12,450 total  |  1,890 this month  |  ▲ 23% vs last month
```

Trend indicators help distinguish actively growing plugins from stale ones.

### Compatibility Matrix

```
Compatibility:
  zhi v0.8+     ✓ tested
  zhi v0.7      ✓ tested
  zhi v0.6      ⚠ untested (may work)
  zhi v0.5      ✗ incompatible
```

Plugin authors declare `minZhiVersion` in their manifest. The marketplace tests compatibility by running the plugin's test suite against multiple zhi versions (if the plugin provides tests).

### Dependent Count

```
Used by: 5 workspaces on marketplace
```

Plugins used by many workspaces are more likely to be maintained and reliable.

### Last Updated

```
Last updated: 3 weeks ago (v1.2.0)
Previous:     3 months ago (v1.1.0)
```

Recency of updates indicates active maintenance.

### Security Audit Badge

For high-value plugins (stores, transforms that handle secrets), an optional security audit badge:

```
Security: Audited by [Auditor Name] on 2025-12-01
```

This is a premium signal — not expected for most community plugins, but valuable for enterprise adoption.

## Community Features

### Plugin Collections

Users can create and share curated collections:

```
Collection: "Kubernetes DevOps Toolkit"
by alice (12 ★)

Plugins:
  - zhi-project/structured-config  ★4.8  config
  - zhi-project/vault-store        ★4.9  store
  - k8s-community/k8s-transform    ★4.5  transform
  - zhi-project/tui                ★4.6  ui

Description: Everything you need for managing Kubernetes
cluster configurations with Vault-backed secret storage.
```

### Publisher Profiles

```
Publisher: zhi-project ★ (Official)
  Plugins: 8
  Workspaces: 3
  Total downloads: 142,000
  Member since: 2025-06-01
  Website: https://zhi.dev
```

### Plugin Discussions

For deeper feedback than ratings allow, each plugin has a discussions section (linked to GitHub Discussions or a built-in forum):

- Bug reports and feature requests
- Usage questions
- Tips and tricks from other users
- Migration guides between versions

The marketplace links to the plugin's GitHub Issues/Discussions rather than reimplementing a forum.

## Abuse Prevention

### Rate Limiting

- Rating submissions: 10 per hour per user
- Helpfulness votes: 50 per hour per user
- Search queries: 100 per minute per IP (unauthenticated), 1000 per minute (authenticated)

### Anti-Spam

- New accounts cannot rate within the first 24 hours
- Reviews with links are flagged for manual review
- Bulk identical ratings from different accounts are detected and removed
- Publisher self-rating is detected and silently excluded from averages

### Dispute Resolution

Publishers can flag reviews they believe are abusive or inaccurate. Flagged reviews are reviewed by a marketplace moderator (zhi maintainer). The moderator can:
- Remove the review (with notification to the reviewer)
- Dismiss the flag (review stays)
- Add a publisher response that appears below the review
