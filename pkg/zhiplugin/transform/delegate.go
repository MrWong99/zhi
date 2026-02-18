package transform

import (
	"context"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

// Compile-time assertion that DelegatingPlugin implements Plugin.
var _ Plugin = (*DelegatingPlugin)(nil)

// DelegatingPlugin forwards all transform.Plugin calls to Base.
// Embed this struct in your own type and override only the methods you
// need. Use [NewDelegatingPlugin] to create one safely.
type DelegatingPlugin struct {
	Base Plugin
}

// NewDelegatingPlugin creates a DelegatingPlugin that delegates to base.
// It panics if base is nil.
func NewDelegatingPlugin(base Plugin) DelegatingPlugin {
	if base == nil {
		panic("transform.NewDelegatingPlugin: base must not be nil")
	}
	return DelegatingPlugin{Base: base}
}

func (d *DelegatingPlugin) BeforeDisplay(ctx context.Context, tree *config.Tree) error {
	return d.Base.BeforeDisplay(ctx, tree)
}

func (d *DelegatingPlugin) AfterSave(ctx context.Context, tree *config.Tree) error {
	return d.Base.AfterSave(ctx, tree)
}

func (d *DelegatingPlugin) ValidatePolicy(ctx context.Context) (ValidatePolicy, error) {
	return d.Base.ValidatePolicy(ctx)
}
