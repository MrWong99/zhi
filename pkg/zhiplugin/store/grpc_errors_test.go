package store

import (
	"testing"
)

func TestErrorRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		err  error
		// verify is called on the reconstructed error
		verify func(t *testing.T, got error)
	}{
		{
			name: "CASConflict all fields",
			err:  &ErrCASConflict{Path: "db/host", Expected: "v3", Actual: "v5"},
			verify: func(t *testing.T, got error) {
				var e *ErrCASConflict
				if !IsCASConflict(got) {
					t.Fatalf("expected ErrCASConflict, got %T: %v", got, got)
				}
				e = got.(*ErrCASConflict)
				if e.Path != "db/host" {
					t.Errorf("Path = %q, want %q", e.Path, "db/host")
				}
				if e.Expected != "v3" {
					t.Errorf("Expected = %q, want %q", e.Expected, "v3")
				}
				if e.Actual != "v5" {
					t.Errorf("Actual = %q, want %q", e.Actual, "v5")
				}
			},
		},
		{
			name: "CASConflict empty path",
			err:  &ErrCASConflict{Expected: "v1", Actual: "v2"},
			verify: func(t *testing.T, got error) {
				e := got.(*ErrCASConflict)
				if e.Path != "" {
					t.Errorf("Path = %q, want empty", e.Path)
				}
				if e.Expected != "v1" {
					t.Errorf("Expected = %q, want %q", e.Expected, "v1")
				}
			},
		},
		{
			name: "AuthRequired",
			err:  &ErrAuthRequired{Reason: "token expired"},
			verify: func(t *testing.T, got error) {
				if !IsAuthRequired(got) {
					t.Fatalf("expected ErrAuthRequired, got %T: %v", got, got)
				}
				e := got.(*ErrAuthRequired)
				if e.Reason != "token expired" {
					t.Errorf("Reason = %q, want %q", e.Reason, "token expired")
				}
			},
		},
		{
			name: "AccessDenied",
			err:  &ErrAccessDenied{Path: "secret/key", Action: "write"},
			verify: func(t *testing.T, got error) {
				if !IsAccessDenied(got) {
					t.Fatalf("expected ErrAccessDenied, got %T: %v", got, got)
				}
				e := got.(*ErrAccessDenied)
				if e.Path != "secret/key" {
					t.Errorf("Path = %q, want %q", e.Path, "secret/key")
				}
				if e.Action != "write" {
					t.Errorf("Action = %q, want %q", e.Action, "write")
				}
			},
		},
		{
			name: "PathNotFound",
			err:  &ErrPathNotFound{TreeID: "tree-42", Path: "missing/key"},
			verify: func(t *testing.T, got error) {
				if !IsPathNotFound(got) {
					t.Fatalf("expected ErrPathNotFound, got %T: %v", got, got)
				}
				e := got.(*ErrPathNotFound)
				if e.TreeID != "tree-42" {
					t.Errorf("TreeID = %q, want %q", e.TreeID, "tree-42")
				}
				if e.Path != "missing/key" {
					t.Errorf("Path = %q, want %q", e.Path, "missing/key")
				}
			},
		},
		{
			name: "EncryptionNotInitialized",
			err:  &ErrEncryptionNotInitialized{},
			verify: func(t *testing.T, got error) {
				if !IsEncryptionNotInitialized(got) {
					t.Fatalf("expected ErrEncryptionNotInitialized, got %T: %v", got, got)
				}
			},
		},
		{
			name: "NotSupported",
			err:  &ErrNotSupported{Feature: "versioning"},
			verify: func(t *testing.T, got error) {
				if !IsNotSupported(got) {
					t.Fatalf("expected ErrNotSupported, got %T: %v", got, got)
				}
				e := got.(*ErrNotSupported)
				if e.Feature != "versioning" {
					t.Errorf("Feature = %q, want %q", e.Feature, "versioning")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to gRPC status, then back.
			grpcErr := errorToStatus(tt.err)
			got := statusToError(grpcErr)
			tt.verify(t, got)
		})
	}
}

func TestErrorToStatus_Nil(t *testing.T) {
	if got := errorToStatus(nil); got != nil {
		t.Errorf("errorToStatus(nil) = %v, want nil", got)
	}
}

func TestStatusToError_Nil(t *testing.T) {
	if got := statusToError(nil); got != nil {
		t.Errorf("statusToError(nil) = %v, want nil", got)
	}
}
