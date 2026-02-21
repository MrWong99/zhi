package store

import (
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errorToStatus converts a store error to a gRPC status with an
// appropriate code. Structured error fields are preserved as gRPC
// ErrorInfo details so they survive the wire round-trip.
// Unrecognised errors are returned as-is (the gRPC framework will
// map them to codes.Unknown).
func errorToStatus(err error) error {
	if err == nil {
		return nil
	}

	var casErr *ErrCASConflict
	if errors.As(err, &casErr) {
		return statusWithDetail(codes.Aborted, err.Error(), map[string]string{
			"path":     casErr.Path,
			"expected": casErr.Expected,
			"actual":   casErr.Actual,
		})
	}

	var authErr *ErrAuthRequired
	if errors.As(err, &authErr) {
		return statusWithDetail(codes.Unauthenticated, err.Error(), map[string]string{
			"reason": authErr.Reason,
		})
	}

	var accessErr *ErrAccessDenied
	if errors.As(err, &accessErr) {
		return statusWithDetail(codes.PermissionDenied, err.Error(), map[string]string{
			"path":   accessErr.Path,
			"action": accessErr.Action,
		})
	}

	var pathErr *ErrPathNotFound
	if errors.As(err, &pathErr) {
		return statusWithDetail(codes.NotFound, err.Error(), map[string]string{
			"tree_id": pathErr.TreeID,
			"path":    pathErr.Path,
		})
	}

	var encErr *ErrEncryptionNotInitialized
	if errors.As(err, &encErr) {
		return status.Error(codes.FailedPrecondition, err.Error())
	}

	var nsErr *ErrNotSupported
	if errors.As(err, &nsErr) {
		return statusWithDetail(codes.Unimplemented, err.Error(), map[string]string{
			"feature": nsErr.Feature,
		})
	}

	return err
}

// statusWithDetail creates a gRPC status error with an ErrorInfo detail
// carrying structured metadata fields.
func statusWithDetail(code codes.Code, msg string, metadata map[string]string) error {
	st, err := status.New(code, msg).WithDetails(&errdetails.ErrorInfo{
		Reason:   "store_error",
		Domain:   "zhi",
		Metadata: metadata,
	})
	if err != nil {
		// WithDetails failed (shouldn't happen); fall back to plain status.
		return status.Error(code, msg)
	}
	return st.Err()
}

// statusToError converts a gRPC status error back to a structured
// store error based on the status code. If ErrorInfo details are
// present, structured fields are reconstructed from the metadata.
// If the error is not a gRPC status, it is returned as-is.
func statusToError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	md := extractErrorInfoMetadata(st)

	switch st.Code() {
	case codes.Aborted:
		return &ErrCASConflict{
			Path:     md["path"],
			Expected: md["expected"],
			Actual:   md["actual"],
		}
	case codes.Unauthenticated:
		return &ErrAuthRequired{Reason: md["reason"]}
	case codes.PermissionDenied:
		return &ErrAccessDenied{
			Path:   md["path"],
			Action: md["action"],
		}
	case codes.NotFound:
		return &ErrPathNotFound{
			TreeID: md["tree_id"],
			Path:   md["path"],
		}
	case codes.FailedPrecondition:
		return &ErrEncryptionNotInitialized{}
	case codes.Unimplemented:
		return &ErrNotSupported{Feature: md["feature"]}
	default:
		return err
	}
}

// extractErrorInfoMetadata extracts metadata from the first ErrorInfo
// detail in a gRPC status. Returns an empty map if no ErrorInfo is found.
func extractErrorInfoMetadata(st *status.Status) map[string]string {
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok {
			if info.Metadata != nil {
				return info.Metadata
			}
			return map[string]string{}
		}
	}
	// Fallback for statuses without details (e.g. from older clients):
	// put the message in common fields so reconstruction is best-effort.
	return map[string]string{}
}
