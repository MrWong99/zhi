// zhi-ui-webui is an external plugin that serves the zhi Web UI.
// It implements ui.Plugin and starts an HTML-based configuration browser.
//
// The listen address defaults to "127.0.0.1:8080" and can be overridden
// with the ZHI_WEBUI_ADDR environment variable.
package main

import (
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/ui/webui"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

func main() {
	cfg := webui.DefaultConfig()
	w := webui.New(cfg)

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ui": &ui.GRPCPlugin{Impl: w},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
	})
}
