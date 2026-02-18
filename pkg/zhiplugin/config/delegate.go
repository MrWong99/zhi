package config

import "context"

// Compile-time assertion that DelegatingPlugin implements Plugin.
var _ Plugin = (*DelegatingPlugin)(nil)

// DelegatingPlugin forwards all config.Plugin calls to Base.
// Embed this struct in your own type and override only the methods you
// need. Use [NewDelegatingPlugin] to create one safely.
type DelegatingPlugin struct {
	Base Plugin
}

// NewDelegatingPlugin creates a DelegatingPlugin that delegates to base.
// It panics if base is nil.
func NewDelegatingPlugin(base Plugin) DelegatingPlugin {
	if base == nil {
		panic("config.NewDelegatingPlugin: base must not be nil")
	}
	return DelegatingPlugin{Base: base}
}

func (d *DelegatingPlugin) List(ctx context.Context) ([]string, error) {
	return d.Base.List(ctx)
}

func (d *DelegatingPlugin) Get(ctx context.Context, path string) (Value, bool, error) {
	return d.Base.Get(ctx, path)
}

func (d *DelegatingPlugin) Set(ctx context.Context, path string, v Value) error {
	return d.Base.Set(ctx, path, v)
}

func (d *DelegatingPlugin) Validate(ctx context.Context, path string, tree TreeReader) ([]ValidationResult, error) {
	return d.Base.Validate(ctx, path, tree)
}
