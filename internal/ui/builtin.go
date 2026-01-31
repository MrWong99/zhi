package ui

import (
	"github.com/MrWong99/zhi/internal/core"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// RegisterBuiltins registers all builtin UI drivers (registered via
// [Register]) into the core [core.Registry] as UI plugin factories.
// This bridges the legacy per-package init() registration with the new
// plugin system.
//
// It should be called after all UI driver init() functions have run (i.e.
// after importing the driver packages) and before resolving UI providers.
func RegisterBuiltins(reg *core.Registry) {
	driversMu.RLock()
	defer driversMu.RUnlock()

	for name, factory := range drivers {
		f := factory // capture loop variable
		_ = reg.RegisterUI(name, func(_ string, _ map[string]any) (zhiui.Plugin, error) {
			return &BuiltinAdapter{Driver: f()}, nil
		})
	}
}
