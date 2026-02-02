package core

// DefaultRegistry returns a registry pre-populated with all built-in
// providers.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	// Errors from registering built-in providers are programming errors
	// (duplicate names), so they are treated as panics.
	if err := r.RegisterConfig("structuredfile", NewStructuredFileProvider); err != nil {
		panic("registering built-in config provider: " + err.Error())
	}
	if err := r.RegisterStore("vault", NewVaultStoreProvider); err != nil {
		panic("registering built-in store provider: " + err.Error())
	}
	return r
}
