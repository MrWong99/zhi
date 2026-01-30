package ui

import (
	"fmt"
	"sort"
	"sync"
)

// UIDriverFactory is a function that creates a new UIDriver instance.
type UIDriverFactory func() UIDriver

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]UIDriverFactory)
)

// Register registers a UI driver factory under the given name.
// It is typically called from an init() function in the driver package.
func Register(name string, factory UIDriverFactory) {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[name] = factory
}

// Get retrieves a UI driver by name, creating a new instance.
func Get(name string) (UIDriver, error) {
	driversMu.RLock()
	defer driversMu.RUnlock()
	factory, ok := drivers[name]
	if !ok {
		available := listLocked()
		return nil, fmt.Errorf("unknown UI driver %q (available: %v)", name, available)
	}
	return factory(), nil
}

// List returns the sorted names of all registered UI drivers.
func List() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	return listLocked()
}

func listLocked() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
