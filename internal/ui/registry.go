package ui

import (
	"fmt"
	"sort"
	"sync"
)

// UIDriverFactory creates a new UIDriver instance.
type UIDriverFactory func() UIDriver

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]UIDriverFactory)
)

// Register registers a UI driver factory under the given name. If a driver
// with the same name already exists, it is overwritten.
func Register(name string, factory UIDriverFactory) {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[name] = factory
}

// Get retrieves a UI driver by name. It returns an error if no driver is
// registered under that name.
func Get(name string) (UIDriver, error) {
	driversMu.RLock()
	defer driversMu.RUnlock()

	factory, ok := drivers[name]
	if !ok {
		available := listLocked()
		if len(available) == 0 {
			return nil, fmt.Errorf("unknown UI driver %q: no drivers registered", name)
		}
		return nil, fmt.Errorf("unknown UI driver %q (available: %v)", name, available)
	}
	return factory(), nil
}

// List returns all registered driver names in sorted order.
func List() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	return listLocked()
}

// listLocked returns sorted driver names. Caller must hold driversMu.
func listLocked() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Reset clears all registered drivers. This is intended for testing only.
func Reset() {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers = make(map[string]UIDriverFactory)
}
