// Package store defines the store plugin API for zhi.
//
// Store plugins persist, retrieve, and delete configuration trees. They may
// optionally support versioning (keeping older versions of trees) and
// encryption at rest.
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
