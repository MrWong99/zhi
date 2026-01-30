package ui_test

import (
	"context"
	"testing"

	"github.com/MrWong99/zhi/internal/ui"
)

type testDriver struct {
	runCalled bool
}

func (d *testDriver) Run(_ context.Context, _ *ui.UIController) error {
	d.runCalled = true
	return nil
}

func TestRegisterAndGet(t *testing.T) {
	// Register a test driver.
	ui.Register("test-driver", func() ui.UIDriver {
		return &testDriver{}
	})

	driver, err := ui.Get("test-driver")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if driver == nil {
		t.Fatal("expected non-nil driver")
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := ui.Get("nonexistent-driver")
	if err == nil {
		t.Error("expected error for unknown driver")
	}
}

func TestList(t *testing.T) {
	// Register another test driver to ensure list works.
	ui.Register("alpha-driver", func() ui.UIDriver {
		return &testDriver{}
	})

	names := ui.List()
	if len(names) == 0 {
		t.Error("expected at least one registered driver")
	}

	// Check sorted order.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("list not sorted: %v", names)
			break
		}
	}
}

func TestRegisterOverwrite(t *testing.T) {
	// Register same name twice - should overwrite.
	ui.Register("overwrite-driver", func() ui.UIDriver {
		return &testDriver{}
	})
	ui.Register("overwrite-driver", func() ui.UIDriver {
		return &testDriver{}
	})

	driver, err := ui.Get("overwrite-driver")
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if driver == nil {
		t.Fatal("expected non-nil driver after overwrite")
	}
}
