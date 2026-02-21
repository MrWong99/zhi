// zhi-store-json is an example zhi store plugin that persists configuration
// trees as JSON files on disk. It delegates to the jsonfile provider in
// pkg/providers/store/jsonfile.
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/providers/store/jsonfile"
	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
)

func main() {
	level := hclog.LevelFromString(os.Getenv("ZHI_LOG_LEVEL"))
	if level == hclog.NoLevel {
		level = hclog.Info
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "zhi-store-json",
		Level:  level,
		Output: os.Stderr,
	})
	logger.Info("starting JSON store plugin")

	dir := os.Getenv("ZHI_JSON_STORE_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "zhi-pokedex")
	}

	s, err := jsonfile.New(jsonfile.Config{Dir: dir})
	if err != nil {
		log.Fatalf("jsonfile.New: %v", err)
	}

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: zhiplugin.Handshake,
		Plugins: map[string]goplugin.Plugin{
			"store": &store.GRPCPlugin{Impl: s},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
