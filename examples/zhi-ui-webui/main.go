// zhi-ui-webui is an external plugin that serves the zhi Web UI.
// It implements ui.Plugin and starts an HTML-based configuration browser.
//
// The listen address defaults to "127.0.0.1:8080" and can be overridden
// with the ZHI_WEBUI_ADDR environment variable.
package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/ui/webui"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-ui-webui",
		Level:  level,
		Output: os.Stderr,
	})
	logger.Info("starting web UI plugin")

	cfg := webui.DefaultConfig()
	w := webui.New(cfg)

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ui": &ui.GRPCPlugin{Impl: w},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
