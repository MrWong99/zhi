package ui_test

import (
	"context"
	"testing"

	"github.com/MrWong99/zhi/internal/ui"
)

// stubDriver is a minimal UIDriver for registry tests.
type stubDriver struct {
	name string
}

func (d *stubDriver) Run(context.Context, *ui.UIController) error { return nil }

func TestRegistry_RegisterAndGet(t *testing.T) {
	ui.Reset()
	defer ui.Reset()

	ui.Register("test-driver", func() ui.UIDriver {
		return &stubDriver{name: "test"}
	})

	driver, err := ui.Get("test-driver")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if driver == nil {
		t.Fatal("expected non-nil driver")
	}

	sd, ok := driver.(*stubDriver)
	if !ok {
		t.Fatal("expected *stubDriver")
	}
	if sd.name != "test" {
		t.Errorf("expected name 'test', got %q", sd.name)
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	ui.Reset()
	defer ui.Reset()

	_, err := ui.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
}

func TestRegistry_GetUnknownWithAvailable(t *testing.T) {
	ui.Reset()
	defer ui.Reset()

	ui.Register("tui", func() ui.UIDriver {
		return &stubDriver{name: "tui"}
	})

	_, err := ui.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown driver")
	}
	// Error message should mention available drivers.
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRegistry_List(t *testing.T) {
	ui.Reset()
	defer ui.Reset()

	ui.Register("beta", func() ui.UIDriver {
		return &stubDriver{name: "beta"}
	})
	ui.Register("alpha", func() ui.UIDriver {
		return &stubDriver{name: "alpha"}
	})

	names := ui.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 drivers, got %d", len(names))
	}
	// Names should be sorted.
	if names[0] != "alpha" {
		t.Errorf("expected first driver 'alpha', got %q", names[0])
	}
	if names[1] != "beta" {
		t.Errorf("expected second driver 'beta', got %q", names[1])
	}
}

func TestRegistry_ListEmpty(t *testing.T) {
	ui.Reset()
	defer ui.Reset()

	names := ui.List()
	if len(names) != 0 {
		t.Errorf("expected 0 drivers, got %d", len(names))
	}
}

func TestRegistry_RegisterDuplicateOverwrites(t *testing.T) {
	ui.Reset()
	defer ui.Reset()

	ui.Register("test", func() ui.UIDriver {
		return &stubDriver{name: "first"}
	})
	ui.Register("test", func() ui.UIDriver {
		return &stubDriver{name: "second"}
	})

	driver, err := ui.Get("test")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	sd, ok := driver.(*stubDriver)
	if !ok {
		t.Fatal("expected *stubDriver")
	}
	if sd.name != "second" {
		t.Errorf("expected overwritten driver 'second', got %q", sd.name)
	}

	// Should still have just one entry.
	names := ui.List()
	if len(names) != 1 {
		t.Errorf("expected 1 driver after overwrite, got %d", len(names))
	}
}
