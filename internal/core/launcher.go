package core

import (
	"fmt"
	"os/exec"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// LaunchConfig launches an external config plugin binary and returns the
// config.Plugin interface along with a cleanup function that kills the
// plugin process. The caller must call cleanup when done.
func LaunchConfig(path string) (config.Plugin, func(), error) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          config.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to config plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense("config")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing config plugin %s: %w", path, err)
	}

	p, ok := raw.(config.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("config plugin %s: dispensed value does not implement config.Plugin", path)
	}

	return p, client.Kill, nil
}

// LaunchTransform launches an external transform plugin binary and returns
// the transform.Plugin interface along with a cleanup function.
func LaunchTransform(path string) (transform.Plugin, func(), error) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          transform.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to transform plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense("transform")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing transform plugin %s: %w", path, err)
	}

	p, ok := raw.(transform.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("transform plugin %s: dispensed value does not implement transform.Plugin", path)
	}

	return p, client.Kill, nil
}

// LaunchStore launches an external store plugin binary and returns the
// store.Plugin interface along with a cleanup function.
func LaunchStore(path string) (store.Plugin, func(), error) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          store.PluginMap,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to store plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense("store")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing store plugin %s: %w", path, err)
	}

	p, ok := raw.(store.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("store plugin %s: dispensed value does not implement store.Plugin", path)
	}

	return p, client.Kill, nil
}
