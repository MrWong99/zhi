package store

import (
	"errors"
	"fmt"
)

// Sentinel errors for common store failures. Plugin implementations
// should return these (or wrap them) so that the engine and UI can
// present meaningful feedback without string-matching.

// ErrCASConflict indicates a compare-and-swap version mismatch.
// The write was rejected because the current version does not match
// the expected version supplied in PutOptions.
type ErrCASConflict struct {
	// Path is the value path that conflicted, or empty for tree-level CAS.
	Path string
	// Expected is the version the caller expected.
	Expected string
	// Actual is the version the store currently holds (may be empty if unknown).
	Actual string
}

func (e *ErrCASConflict) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("CAS conflict on %q: expected version %q, actual %q", e.Path, e.Expected, e.Actual)
	}
	return fmt.Sprintf("CAS conflict: expected version %q, actual %q", e.Expected, e.Actual)
}

// ErrAuthRequired indicates that authentication is required or the
// current session has expired.
type ErrAuthRequired struct {
	// Reason provides additional context (e.g. "token expired").
	Reason string
}

func (e *ErrAuthRequired) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("authentication required: %s", e.Reason)
	}
	return "authentication required"
}

// ErrAccessDenied indicates that the authenticated identity does not
// have sufficient permissions for the requested operation.
type ErrAccessDenied struct {
	// Path is the path access was denied for, or empty for tree-level operations.
	Path string
	// Action is the operation that was denied (e.g. "read", "write", "delete").
	Action string
}

func (e *ErrAccessDenied) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("access denied: %s on %q", e.Action, e.Path)
	}
	return fmt.Sprintf("access denied: %s", e.Action)
}

// ErrPathNotFound indicates that the requested path does not exist in
// the tree.
type ErrPathNotFound struct {
	// TreeID is the tree that was searched.
	TreeID string
	// Path is the path that was not found.
	Path string
}

func (e *ErrPathNotFound) Error() string {
	return fmt.Sprintf("path %q not found in tree %q", e.Path, e.TreeID)
}

// ErrEncryptionNotInitialized indicates that an operation requiring
// encryption was attempted before encryption was initialized.
type ErrEncryptionNotInitialized struct{}

func (e *ErrEncryptionNotInitialized) Error() string {
	return "encryption not initialized"
}

// ErrNotSupported indicates that a feature is not supported by this
// store plugin. This is returned by methods that do not apply to the
// plugin's capabilities (e.g. versioning methods on a non-versioning store).
type ErrNotSupported struct {
	// Feature describes what is not supported (e.g. "versioning", "encryption").
	Feature string
}

func (e *ErrNotSupported) Error() string {
	return fmt.Sprintf("%s not supported", e.Feature)
}

// Helper functions for checking error types.

// IsCASConflict reports whether err (or any error in its chain) is an
// ErrCASConflict.
func IsCASConflict(err error) bool {
	var target *ErrCASConflict
	return errors.As(err, &target)
}

// IsAuthRequired reports whether err (or any error in its chain) is an
// ErrAuthRequired.
func IsAuthRequired(err error) bool {
	var target *ErrAuthRequired
	return errors.As(err, &target)
}

// IsAccessDenied reports whether err (or any error in its chain) is an
// ErrAccessDenied.
func IsAccessDenied(err error) bool {
	var target *ErrAccessDenied
	return errors.As(err, &target)
}

// IsPathNotFound reports whether err (or any error in its chain) is an
// ErrPathNotFound.
func IsPathNotFound(err error) bool {
	var target *ErrPathNotFound
	return errors.As(err, &target)
}

// IsEncryptionNotInitialized reports whether err (or any error in its
// chain) is an ErrEncryptionNotInitialized.
func IsEncryptionNotInitialized(err error) bool {
	var target *ErrEncryptionNotInitialized
	return errors.As(err, &target)
}

// IsNotSupported reports whether err (or any error in its chain) is an
// ErrNotSupported.
func IsNotSupported(err error) bool {
	var target *ErrNotSupported
	return errors.As(err, &target)
}
