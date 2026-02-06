package client

import (
	"fmt"
	"strings"
)

// ParsedRef holds the components of an OCI reference.
type ParsedRef struct {
	// Host is the registry host (e.g. "ghcr.io").
	Host string
	// Repository is the image repository path (e.g. "zhi-project/zhi-config-ansible").
	Repository string
	// Tag is the image tag (e.g. "v1.2.0"). Empty if digest is specified.
	Tag string
	// Digest is the content digest (e.g. "sha256:abc123"). Empty if tag is specified.
	Digest string
}

// Reference returns the full reference string in the form host/repo:tag or host/repo@digest.
func (r *ParsedRef) Reference() string {
	ref := r.Host + "/" + r.Repository
	if r.Digest != "" {
		return ref + "@" + r.Digest
	}
	if r.Tag != "" {
		return ref + ":" + r.Tag
	}
	return ref
}

// IsShortName returns true when the input looks like a marketplace short name
// (e.g. "ansible-config" or "ansible-config@1.2.0") rather than a full OCI
// reference. A short name contains no slashes and no "oci://" scheme prefix.
func IsShortName(ref string) bool {
	if strings.HasPrefix(ref, "oci://") {
		return false
	}
	// A short name has no slashes (OCI refs always have host/repo).
	return !strings.Contains(ref, "/")
}

// ParseReference parses an OCI reference string. It accepts:
//   - oci://host/repo:tag
//   - oci://host/repo@digest
//   - host/repo:tag (without scheme)
//   - host/repo@digest (without scheme)
func ParseReference(ref string) (*ParsedRef, error) {
	// Strip oci:// scheme if present.
	raw := strings.TrimPrefix(ref, "oci://")

	if raw == "" {
		return nil, fmt.Errorf("empty OCI reference")
	}

	parsed := &ParsedRef{}

	// Split by @ for digest references.
	if idx := strings.Index(raw, "@"); idx >= 0 {
		parsed.Digest = raw[idx+1:]
		raw = raw[:idx]
	}

	// Split by : for tag (only if no digest and host already separated).
	if parsed.Digest == "" {
		// Find the last colon that's after the first slash (to avoid matching port numbers).
		slashIdx := strings.Index(raw, "/")
		if slashIdx < 0 {
			return nil, fmt.Errorf("invalid OCI reference %q: missing repository path", ref)
		}
		colonIdx := strings.LastIndex(raw[slashIdx:], ":")
		if colonIdx >= 0 {
			colonIdx += slashIdx
			parsed.Tag = raw[colonIdx+1:]
			raw = raw[:colonIdx]
		}
	}

	// Split host from repository.
	before, after, ok := strings.Cut(raw, "/")
	if !ok {
		return nil, fmt.Errorf("invalid OCI reference %q: missing repository path", ref)
	}
	parsed.Host = before
	parsed.Repository = after

	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid OCI reference %q: empty host", ref)
	}
	if parsed.Repository == "" {
		return nil, fmt.Errorf("invalid OCI reference %q: empty repository", ref)
	}

	return parsed, nil
}
