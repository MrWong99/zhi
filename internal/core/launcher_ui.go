package core

import (
	"fmt"
	"os/exec"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	zhiui "github.com/MrWong99/zhi/pkg/zhiplugin/ui"
)

// launchUI launches an external UI plugin binary. UI plugins use the
// go-plugin broker for bidirectional gRPC (Controller callbacks), which
// is not supported by the public launch package.
func launchUI(path string) (zhiui.Plugin, func(), error) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          zhiui.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to UI plugin %s: %w", path, err)
	}

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

	return p, client.Kill, nil
}
