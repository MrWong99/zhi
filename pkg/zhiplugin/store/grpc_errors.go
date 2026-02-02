package store

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errorToStatus converts a store error to a gRPC status with an
// appropriate code. Unrecognised errors are returned as-is (the gRPC
// framework will map them to codes.Unknown).
func errorToStatus(err error) error {
	if err == nil {
		return nil
	}

	var casErr *ErrCASConflict
	if errors.As(err, &casErr) {
		return status.Error(codes.Aborted, err.Error())
	}

	var authErr *ErrAuthRequired
	if errors.As(err, &authErr) {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	var accessErr *ErrAccessDenied
	if errors.As(err, &accessErr) {
		return status.Error(codes.PermissionDenied, err.Error())
	}

	var pathErr *ErrPathNotFound
	if errors.As(err, &pathErr) {
		return status.Error(codes.NotFound, err.Error())
	}

	var encErr *ErrEncryptionNotInitialized
	if errors.As(err, &encErr) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}

	var nsErr *ErrNotSupported
	if errors.As(err, &nsErr) {
		return status.Error(codes.Unimplemented, err.Error())
	}

	return err
}

// statusToError converts a gRPC status error back to a structured
// store error based on the status code and message. If the error is
// not a gRPC status, it is returned as-is.
func statusToError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	msg := st.Message()
	switch st.Code() {
	case codes.Aborted:
		// Attempt to reconstruct a CASConflict. We preserve the
		// original message in the struct so callers can inspect it.
		return &ErrCASConflict{Expected: "", Actual: "", Path: msg}
	case codes.Unauthenticated:
		return &ErrAuthRequired{Reason: msg}
	case codes.PermissionDenied:
		return &ErrAccessDenied{Action: msg}
	case codes.NotFound:
		return &ErrPathNotFound{Path: msg}
	case codes.FailedPrecondition:
		return &ErrEncryptionNotInitialized{}
	case codes.Unimplemented:
		return &ErrNotSupported{Feature: msg}
	default:
		return err
	}
}
