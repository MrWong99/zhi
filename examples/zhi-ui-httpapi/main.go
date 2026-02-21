// zhi-ui-httpapi is an example zhi UI plugin that exposes the
// configuration engine as an HTTP/JSON API. It delegates to the httpapi
// provider in pkg/providers/ui/httpapi.
package main

import (
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/ui/httpapi"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-ui-httpapi",
		Level:  level,
		Output: os.Stderr,
	})
	logger.Info("starting HTTP API UI plugin")

	addr := os.Getenv("ZHI_HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	p, err := httpapi.New(httpapi.Config{Addr: addr})
	if err != nil {
		log.Fatalf("httpapi.New: %v", err)
	}

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"ui": &ui.GRPCPlugin{Impl: p},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
