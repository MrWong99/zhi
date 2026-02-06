// Package server implements the HTTP handlers and policy engine for the
// zhi-mirror registry proxy.
package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Policy defines the mirror-level policy that controls what artifacts can be
// pulled through the mirror. It is loaded from a YAML file
// (e.g. /etc/zhi-mirror/policy.yaml).
type Policy struct {
	// AllowedPublishers limits pulls to artifacts from these publishers.
	// An empty list allows all publishers.
	AllowedPublishers []string `yaml:"allowedPublishers,omitempty" json:"allowedPublishers,omitempty"`
	// BlockedArtifacts is a list of artifact references that are blocked.
	BlockedArtifacts []string `yaml:"blockedArtifacts,omitempty" json:"blockedArtifacts,omitempty"`
	// AllowedTypes limits which plugin types can be pulled (config, transform, store, ui).
	// An empty list allows all types.
	AllowedTypes []string `yaml:"allowedTypes,omitempty" json:"allowedTypes,omitempty"`
	// RequireSignatures rejects unsigned artifacts even if the publisher is allowed.
	RequireSignatures bool `yaml:"requireSignatures" json:"requireSignatures"`
	// Sync defines artifacts to pre-populate into the cache on a schedule.
	Sync []SyncRule `yaml:"sync,omitempty" json:"sync,omitempty"`
	// Retention controls automatic cleanup of cached artifacts.
	Retention RetentionPolicy `yaml:"retention,omitempty" json:"retention"`
}

// SyncRule defines an artifact reference pattern and a cron schedule for
// automatic pre-population of the mirror cache.
type SyncRule struct {
	// Ref is an OCI reference pattern (e.g. "oci://ghcr.io/zhi-project/zhi-config-ansible:v1.*").
	Ref string `yaml:"ref" json:"ref"`
	// Schedule is a cron expression (e.g. "0 2 * * *" for daily at 2 AM).
	Schedule string `yaml:"schedule" json:"schedule"`
}

// RetentionPolicy controls automatic cleanup of cached artifacts.
type RetentionPolicy struct {
	// UnusedDays removes artifacts not pulled in this many days.
	UnusedDays int `yaml:"unusedDays,omitempty" json:"unusedDays,omitempty"`
	// MaxStorage is the hard cap on storage (e.g. "50GB"). LRU eviction.
	MaxStorage string `yaml:"maxStorage,omitempty" json:"maxStorage,omitempty"`
}

// DefaultPolicy returns a permissive policy that allows everything.
func DefaultPolicy() *Policy {
	return &Policy{}
}

// LoadPolicy reads a policy from a YAML file. Returns a default policy if
// the file does not exist.
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPolicy(), nil
		}
		return nil, fmt.Errorf("reading policy file: %w", err)
	}
	return ParsePolicy(data)
}

// ParsePolicy parses policy YAML data.
func ParsePolicy(data []byte) (*Policy, error) {
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing policy YAML: %w", err)
	}
	return &p, nil
}

// DefaultPolicyPath returns the default policy file path.
func DefaultPolicyPath() string {
	return filepath.Join("/etc", "zhi-mirror", "policy.yaml")
}

// IsPublisherAllowed checks whether a publisher is allowed by the policy.
// If no allowed publishers are configured, all publishers are allowed.
func (p *Policy) IsPublisherAllowed(publisher string) bool {
	if len(p.AllowedPublishers) == 0 {
		return true
	}
	for _, allowed := range p.AllowedPublishers {
		if strings.EqualFold(allowed, publisher) {
			return true
		}
	}
	return false
}

// IsArtifactBlocked checks whether a repository reference is blocked.
func (p *Policy) IsArtifactBlocked(ref string) bool {
	ref = strings.TrimPrefix(ref, "oci://")
	for _, blocked := range p.BlockedArtifacts {
		if strings.Contains(ref, blocked) {
			return true
		}
	}
	return false
}

// IsTypeAllowed checks whether a plugin type is allowed.
// If no allowed types are configured, all types are allowed.
func (p *Policy) IsTypeAllowed(pluginType string) bool {
	if len(p.AllowedTypes) == 0 {
		return true
	}
	for _, allowed := range p.AllowedTypes {
		if strings.EqualFold(allowed, pluginType) {
			return true
		}
	}
	return false
}

// CheckRepository evaluates policy against a repository path. The repository
// path is in the form "publisher/name" (e.g. "zhi-project/zhi-config-ansible").
// Returns nil if allowed, or an error describing why it was rejected.
func (p *Policy) CheckRepository(repo string) error {
	if p.IsArtifactBlocked(repo) {
		return fmt.Errorf("artifact %q is blocked by policy", repo)
	}
	publisher := extractPublisher(repo)
	if !p.IsPublisherAllowed(publisher) {
		return fmt.Errorf("publisher %q is not allowed by policy", publisher)
	}
	return nil
}

// extractPublisher returns the first path component (publisher) from a
// repository path like "zhi-project/zhi-config-ansible".
func extractPublisher(repo string) string {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
