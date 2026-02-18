package core

import (
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/launch"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// engineLaunchOpts returns the standard launch options for the engine.
// The engine uses AuditWarnOnly to preserve its historical behavior of
// logging integrity failures rather than hard-failing. Meta-plugins
// calling the launch package directly get the stricter AuditHardFail
// default.
func engineLaunchOpts() []launch.Option {
	return []launch.Option{
		launch.WithLogger(Logger()),
		launch.WithAuditMode(launch.AuditWarnOnly),
	}
}

// LaunchConfig launches an external config plugin binary and returns the
// config.Plugin interface along with a cleanup function that kills the
// plugin process. The caller must call cleanup when done.
func LaunchConfig(path string) (config.Plugin, func(), error) {
	return launch.LaunchConfig(path, engineLaunchOpts()...)
}

// LaunchTransform launches an external transform plugin binary and returns
// the transform.Plugin interface along with a cleanup function.
func LaunchTransform(path string) (transform.Plugin, func(), error) {
	return launch.LaunchTransform(path, engineLaunchOpts()...)
}

// LaunchStore launches an external store plugin binary and returns the
// store.Plugin interface along with a cleanup function.
func LaunchStore(path string) (store.Plugin, func(), error) {
	return launch.LaunchStore(path, engineLaunchOpts()...)
}

// LaunchUI launches an external UI plugin binary and returns the
// ui.Plugin interface along with a cleanup function.
//
// Note: The launch package does not support UI plugins because they use
// bidirectional gRPC via the go-plugin broker. This function preserves
// the original implementation for internal use.
func LaunchUI(path string) (zhiui.Plugin, func(), error) {
	return launchUI(path)
}
