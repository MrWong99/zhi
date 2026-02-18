package core

import (
	"github.com/MrWong99/zhi/pkg/zhiplugin/launch"
)

// auditPluginBinary is a backward-compatible wrapper that delegates to the
// public launch package. It uses the internal Logger() for reporting.
func auditPluginBinary(path string) {
	_ = launch.AuditBinary(Logger(), path)
}

// pluginNameFromPath is a backward-compatible wrapper that delegates to the
// public launch package.
func pluginNameFromPath(path string) string {
	return launch.PluginNameFromPath(path)
}
