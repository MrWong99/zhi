package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Policy defines the security policy for plugin verification. It is
// loaded from ~/.zhi/policy.yaml or configured inline in
// ~/.zhi/config.yaml under the "sharing.verification" key.
type Policy struct {
	// RequireSignatures enables strict mode (Level 3): only signed
	// artifacts from trusted publishers are allowed.
	RequireSignatures bool `yaml:"requireSignatures" json:"requireSignatures"`
	// TrustedPublishers is a list of publisher identities that are
	// automatically trusted (e.g. "zhi-project", "myorg").
	TrustedPublishers []string `yaml:"trustedPublishers,omitempty" json:"trustedPublishers,omitempty"`
	// AllowedRegistries limits which OCI registries plugins can be
	// installed from. An empty list allows all registries.
	AllowedRegistries []string `yaml:"allowedRegistries,omitempty" json:"allowedRegistries,omitempty"`
	// BlockedPlugins is a list of plugin references that are blocked
	// from installation (e.g. "untrusted/bad-plugin").
	BlockedPlugins []string `yaml:"blockedPlugins,omitempty" json:"blockedPlugins,omitempty"`
}

// policyFile is the top-level YAML structure of ~/.zhi/policy.yaml.
type policyFile struct {
	Sharing struct {
		Verification Policy `yaml:"verification"`
	} `yaml:"sharing"`
}

// DefaultPolicy returns a permissive policy that accepts all artifacts.
func DefaultPolicy() *Policy {
	return &Policy{}
}

// LoadPolicyFile reads a policy from a YAML file at the given path.
// Returns a default policy if the file does not exist.
func LoadPolicyFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPolicy(), nil
		}
		return nil, fmt.Errorf("reading policy file: %w", err)
	}
	return ParsePolicy(data)
}

// ParsePolicy parses policy YAML data. The data can be either a full
// policy file (with sharing.verification wrapper) or a bare Policy object.
func ParsePolicy(data []byte) (*Policy, error) {
	// Try the full wrapper format first.
	var pf policyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing policy YAML: %w", err)
	}
	p := pf.Sharing.Verification
	// If the wrapper parsed but was empty, try bare format.
	if !p.RequireSignatures && len(p.TrustedPublishers) == 0 &&
		len(p.AllowedRegistries) == 0 && len(p.BlockedPlugins) == 0 {
		var bare Policy
		if err := yaml.Unmarshal(data, &bare); err == nil && (bare.RequireSignatures || len(bare.TrustedPublishers) > 0 || len(bare.AllowedRegistries) > 0 || len(bare.BlockedPlugins) > 0) {
			return &bare, nil
		}
	}
	return &p, nil
}

// DefaultPolicyPath returns the default path to the policy file (~/.zhi/policy.yaml).
func DefaultPolicyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zhi", "policy.yaml")
}

// EffectiveLevel returns the verification level implied by this policy.
func (p *Policy) EffectiveLevel() Level {
	if p.RequireSignatures {
		return LevelStrict
	}
	return LevelSigned
}

// IsTrustedPublisher checks whether the given publisher identity is in
// the trusted publishers list.
func (p *Policy) IsTrustedPublisher(identity string) bool {
	for _, tp := range p.TrustedPublishers {
		if strings.EqualFold(tp, identity) {
			return true
		}
	}
	return false
}

// IsRegistryAllowed checks whether a reference's registry host is allowed.
// If no allowed registries are configured, all registries are allowed.
func (p *Policy) IsRegistryAllowed(ref string) bool {
	if len(p.AllowedRegistries) == 0 {
		return true
	}
	host := extractRegistryHost(ref)
	for _, allowed := range p.AllowedRegistries {
		if strings.EqualFold(host, allowed) {
			return true
		}
	}
	return false
}

// IsBlocked checks whether a reference matches a blocked plugin pattern.
// Patterns match whole path segments of the repository (tag and digest are
// ignored), so blocking "org/plugin" does not block "org/plugin-fork" or
// "other-org/pluginish".
func (p *Policy) IsBlocked(ref string) bool {
	ref = strings.TrimPrefix(ref, "oci://")
	// Strip a trailing digest or tag so patterns match the repository path.
	if i := strings.LastIndex(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		ref = ref[:i]
	}
	segments := strings.Split(ref, "/")
	for _, blocked := range p.BlockedPlugins {
		pattern := strings.Split(strings.TrimPrefix(blocked, "oci://"), "/")
		if matchesSegments(segments, pattern) {
			return true
		}
	}
	return false
}

// matchesSegments reports whether pattern occurs in segments as a
// contiguous run of whole path segments.
func matchesSegments(segments, pattern []string) bool {
	if len(pattern) == 0 {
		return false
	}
	for i := 0; i+len(pattern) <= len(segments); i++ {
		matched := true
		for j, p := range pattern {
			if segments[i+j] != p {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
