// Package store defines the store plugin API for zhi.
//
// Store plugins persist, retrieve, and delete configuration values within
// named trees. The API is designed with security-first principles:
// operations are granular down to individual values, authentication and
// access control hooks are built in, and versioning supports both
// tree-level and value-level flavors.
package store

// EncryptionStatus reports the encryption state of a store plugin.
type EncryptionStatus int

const (
	// EncryptionNone means the store does not support encryption at rest.
	EncryptionNone EncryptionStatus = iota
	// EncryptionSupported means the store supports encryption but it has
	// not been initialized yet.
	EncryptionSupported
	// EncryptionActive means encryption is initialized and data is being
	// encrypted at rest.
	EncryptionActive
)

func (s EncryptionStatus) String() string {
	switch s {
	case EncryptionNone:
		return "none"
	case EncryptionSupported:
		return "supported"
	case EncryptionActive:
		return "active"
	default:
		return "unknown"
	}
}

// VersioningMode describes how a store plugin versions data.
type VersioningMode int

const (
	// VersioningNone means the store does not support versioning.
	VersioningNone VersioningMode = iota
	// VersioningTree means the entire tree is versioned atomically.
	// A save creates a new version of the whole tree, and rollbacks
	// restore the entire tree.
	VersioningTree
	// VersioningValue means individual values are versioned independently.
	// Each value has its own version history and can be rolled back
	// individually.
	VersioningValue
)

func (m VersioningMode) String() string {
	switch m {
	case VersioningNone:
		return "none"
	case VersioningTree:
		return "tree"
	case VersioningValue:
		return "value"
	default:
		return "unknown"
	}
}

// Capabilities reports what features a store plugin supports.
type Capabilities struct {
	// Versioning describes the versioning strategy.
	Versioning VersioningMode
	// Encryption reports the current encryption state.
	Encryption EncryptionStatus
	// Auth is true if the store requires or supports authentication.
	Auth bool
	// AccessControl is true if the store supports per-value access
	// control and sharing.
	AccessControl bool
}

// AuthMethod describes an authentication method supported by the store.
type AuthMethod struct {
	// Type is the method identifier, e.g. "userpass", "token", "oidc".
	Type string
	// Description is a human-readable description of the method.
	Description string
	// Fields describes the credential fields needed for this method.
	Fields []AuthField
}

// AuthField describes a single credential field for an auth method.
type AuthField struct {
	// Name is the field identifier, e.g. "username", "password", "token".
	Name string
	// Description is a human-readable label for the field.
	Description string
	// Required is true if this field must be provided.
	Required bool
	// Secret is true if this field should be masked in UIs (e.g. passwords).
	Secret bool
}

// Credential represents an opaque authentication credential returned
// after a successful login.
type Credential struct {
	// Token is an opaque session or access token.
	Token string
	// ExpiresAt is an optional RFC3339 timestamp indicating expiry.
	ExpiresAt string
	// Metadata carries optional key-value information about the session.
	Metadata map[string]string
}

// PutOptions controls optional behaviour of PutValues.
type PutOptions struct {
	// CASVersion is the expected current tree version for tree-level CAS.
	// If non-empty and the current version differs, the write fails with
	// an error. Only meaningful when versioning mode is VersioningTree.
	CASVersion string
	// CASVersions maps individual paths to their expected current
	// versions. If a path's current version differs, the write for that
	// path fails. Only meaningful when versioning mode is VersioningValue.
	CASVersions map[string]string
}

// Action describes a type of access to a stored value.
type Action int

const (
	// ActionRead allows reading a value.
	ActionRead Action = iota
	// ActionWrite allows creating or updating a value.
	ActionWrite
	// ActionDelete allows removing a value.
	ActionDelete
)

func (a Action) String() string {
	switch a {
	case ActionRead:
		return "read"
	case ActionWrite:
		return "write"
	case ActionDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// Permission describes access to a specific path or path prefix within
// a tree.
type Permission struct {
	// Path is an exact path (e.g. "database/host") or a path prefix
	// ending in "/" (e.g. "database/") that this permission covers.
	Path string
	// Actions lists the allowed operations.
	Actions []Action
}
