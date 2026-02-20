package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// launchUI launches an external UI plugin binary. UI plugins use the
// go-plugin broker for bidirectional gRPC (Controller callbacks), which
// is not supported by the public launch package.
func launchUI(path string, options map[string]any) (zhiui.Plugin, func(), error) {
	log := Logger()
	pluginName := filepath.Base(path)
	pluginLogger := log.Named("plugin.ui." + pluginName)

	log.Info("launching UI plugin", "binary", path)
	cmd := exec.Command(path)

	// Inject workspace options as JSON in ZHI_PLUGIN_OPTIONS.
	if len(options) > 0 {
		data, err := json.Marshal(options)
		if err != nil {
			return nil, nil, fmt.Errorf("marshalling plugin options: %w", err)
		}
		cmd.Env = append(os.Environ(), "ZHI_PLUGIN_OPTIONS="+string(data))
		log.Debug("injecting plugin options", "binary", path)
	}

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          zhiui.PluginMap,
		Cmd:              cmd,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           pluginLogger,
	})

	log.Debug("connecting to UI plugin process", "binary", path)
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to UI plugin %s: %w", path, err)
	}

	log.Debug("dispensing UI plugin interface", "binary", path)
	raw, err := rpcClient.Dispense("ui")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing UI plugin %s: %w", path, err)
	}

	p, ok := raw.(zhiui.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("UI plugin %s: dispensed value does not implement ui.Plugin", path)
	}

	log.Info("UI plugin ready", "binary", path)
	return p, client.Kill, nil
}
