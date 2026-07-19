package store_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func TestMirroredPlugin_ReadFromPrimary(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	primary.values = map[string]config.Value{"k": {Val: "primary"}}
	mirror := newMockStore()
	mirror.values = map[string]config.Value{"k": {Val: "mirror"}}

	mp := store.MirroredPlugin(nil, primary, mirror)

	vals, err := mp.GetValues(context.Background(), "t1", []string{"k"})
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if vals["k"].Val != "primary" {
		t.Errorf("GetValues returned %v, want 'primary'", vals["k"].Val)
	}
	if !primary.calledMethods["GetValues"] {
		t.Error("GetValues should be called on primary")
	}
	if mirror.calledMethods["GetValues"] {
		t.Error("GetValues should NOT be called on mirror")
	}
}

func TestMirroredPlugin_WriteToBoth(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	mirror := newMockStore()

	mp := store.MirroredPlugin(nil, primary, mirror)

	err := mp.PutValues(context.Background(), "t1", map[string]config.Value{"k": {Val: "v"}}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}
	if !primary.calledMethods["PutValues"] {
		t.Error("PutValues should be called on primary")
	}
	if !mirror.calledMethods["PutValues"] {
		t.Error("PutValues should be called on mirror")
	}
}

func TestMirroredPlugin_PrimaryWriteFailure(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	primary.err = errors.New("primary failed")
	mirror := newMockStore()

	mp := store.MirroredPlugin(nil, primary, mirror)

	err := mp.PutValues(context.Background(), "t1", nil, nil)
	if err == nil {
		t.Fatal("expected error when primary fails")
	}
	// Mirror should NOT be called when primary fails.
	if mirror.calledMethods["PutValues"] {
		t.Error("mirror should not be called when primary fails")
	}
}

func TestMirroredPlugin_MirrorWriteFailureDoesNotFail(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	mirror := newMockStore()
	mirror.err = errors.New("mirror failed")

	mp := store.MirroredPlugin(nil, primary, mirror)

	// PutValues should succeed despite mirror failure.
	err := mp.PutValues(context.Background(), "t1", map[string]config.Value{"k": {Val: "v"}}, nil)
	if err != nil {
		t.Fatalf("PutValues should succeed despite mirror failure: %v", err)
	}
}

func TestMirroredPlugin_DeleteTreeMirrored(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	mirror := newMockStore()

	mp := store.MirroredPlugin(nil, primary, mirror)

	err := mp.DeleteTree(context.Background(), "t1")
	if err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}
	if !primary.calledMethods["DeleteTree"] {
		t.Error("DeleteTree should be called on primary")
	}
	if !mirror.calledMethods["DeleteTree"] {
		t.Error("DeleteTree should be called on mirror")
	}
}

func TestMirroredPlugin_DeleteValuesMirrored(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	mirror := newMockStore()

	mp := store.MirroredPlugin(nil, primary, mirror)

	err := mp.DeleteValues(context.Background(), "t1", []string{"a"})
	if err != nil {
		t.Fatalf("DeleteValues: %v", err)
	}
	if !primary.calledMethods["DeleteValues"] {
		t.Error("DeleteValues should be called on primary")
	}
	if !mirror.calledMethods["DeleteValues"] {
		t.Error("DeleteValues should be called on mirror")
	}
}

func TestMirroredPlugin_RollbackTreeMirrored(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	mirror := newMockStore()

	mp := store.MirroredPlugin(nil, primary, mirror)

	if err := mp.RollbackTree(context.Background(), "t1", "v3"); err != nil {
		t.Fatalf("RollbackTree: %v", err)
	}
	if !primary.calledMethods["RollbackTree"] {
		t.Error("RollbackTree should be called on primary")
	}
	if !mirror.calledMethods["RollbackTree"] {
		t.Error("RollbackTree should be forwarded to mirror to avoid divergence")
	}
}

func TestMirroredPlugin_RollbackValueMirrored(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	mirror := newMockStore()

	mp := store.MirroredPlugin(nil, primary, mirror)

	if err := mp.RollbackValue(context.Background(), "t1", "db/host", "v3"); err != nil {
		t.Fatalf("RollbackValue: %v", err)
	}
	if !primary.calledMethods["RollbackValue"] {
		t.Error("RollbackValue should be called on primary")
	}
	if !mirror.calledMethods["RollbackValue"] {
		t.Error("RollbackValue should be forwarded to mirror to avoid divergence")
	}
}

func TestMirroredPlugin_AuthDelegatesToPrimary(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	mirror := newMockStore()

	mp := store.MirroredPlugin(nil, primary, mirror)
	ctx := context.Background()

	_, _ = mp.AuthMethods(ctx)
	_, _ = mp.Login(ctx, "token", nil)
	_, _ = mp.LoginInteractive(ctx, "oidc", nil)
	_, _ = mp.LoginInteractiveCallback(ctx, "ch1", nil)

	for _, method := range []string{"AuthMethods", "Login", "LoginInteractive", "LoginInteractiveCallback"} {
		if !primary.calledMethods[method] {
			t.Errorf("%s should be called on primary", method)
		}
		if mirror.calledMethods[method] {
			t.Errorf("%s should NOT be called on mirror", method)
		}
	}
}

func TestMirroredPlugin_CapabilitiesReportPrimary(t *testing.T) {
	t.Parallel()
	primary := newMockStore()
	primary.caps = &store.Capabilities{
		Versioning:    store.VersioningTree,
		Encryption:    store.EncryptionActive,
		Auth:          true,
		AccessControl: true,
	}
	mirror := newMockStore()
	mirror.caps = &store.Capabilities{
		Versioning:    store.VersioningNone,
		Encryption:    store.EncryptionNone,
		Auth:          false,
		AccessControl: false,
	}

	mp := store.MirroredPlugin(nil, primary, mirror)

	caps, err := mp.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	// All capability-gated operations delegate to the primary only, so the
	// composed plugin must report the primary's capabilities verbatim
	// rather than intersecting with weaker mirrors.
	if caps.Versioning != store.VersioningTree {
		t.Errorf("Versioning = %v, want VersioningTree (primary)", caps.Versioning)
	}
	if caps.Encryption != store.EncryptionActive {
		t.Errorf("Encryption = %v, want EncryptionActive (primary)", caps.Encryption)
	}
	if !caps.Auth {
		t.Error("Auth should be true (primary)")
	}
	if !caps.AccessControl {
		t.Error("AccessControl should be true (primary)")
	}
}

func TestMirroredPlugin_MirrorWritesParallel(t *testing.T) {
	t.Parallel()
	primary := newMockStore()

	var running atomic.Int32
	var maxRunning atomic.Int32

	// Create multiple mirrors that track concurrency.
	mirrors := make([]store.Plugin, 3)
	for i := range mirrors {
		mirrors[i] = &concurrencyTracker{
			mockStorePlugin: *newMockStore(),
			running:         &running,
			maxRunning:      &maxRunning,
		}
	}

	mp := store.MirroredPlugin(nil, primary, mirrors...)

	err := mp.PutValues(context.Background(), "t1", map[string]config.Value{"k": {Val: "v"}}, nil)
	if err != nil {
		t.Fatalf("PutValues: %v", err)
	}

	// If mirrors ran in parallel, maxRunning should be > 1 (usually 3).
	// We can't guarantee exact parallelism, but we can verify all were called.
	for i, mirror := range mirrors {
		ct := mirror.(*concurrencyTracker)
		if !ct.calledMethods["PutValues"] {
			t.Errorf("mirror %d was not called", i)
		}
	}
}

// concurrencyTracker wraps a mock store to track concurrent execution.
type concurrencyTracker struct {
	mockStorePlugin
	running    *atomic.Int32
	maxRunning *atomic.Int32
}

func (c *concurrencyTracker) PutValues(ctx context.Context, id string, values map[string]config.Value, opts *store.PutOptions) error {
	cur := c.running.Add(1)
	defer c.running.Add(-1)

	// Track max concurrent.
	for {
		old := c.maxRunning.Load()
		if cur <= old || c.maxRunning.CompareAndSwap(old, cur) {
			break
		}
	}

	return c.mockStorePlugin.PutValues(ctx, id, values, opts)
}
