package launch

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/MrWong99/zhi/pkg/zhiplugin"
	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
	"github.com/MrWong99/zhi/pkg/zhiplugin/store"
	"github.com/MrWong99/zhi/pkg/zhiplugin/transform"
)

// LaunchConfig launches an external config plugin binary and returns the
// config.Plugin interface along with a cleanup function that kills the
// plugin process. The caller must call cleanup when done.
func LaunchConfig(binary string, opts ...Option) (config.Plugin, func(), error) {
	o := applyOptions(opts)

	client, err := launchClient(binary, config.PluginMap, o)
	if err != nil {
		return nil, nil, fmt.Errorf("launching config plugin %s: %w", binary, err)
	}

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to config plugin %s: %w", binary, err)
	}

	raw, err := rpcClient.Dispense("config")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing config plugin %s: %w", binary, err)
	}

	p, ok := raw.(config.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("config plugin %s: dispensed value does not implement config.Plugin", binary)
	}

	return p, client.Kill, nil
}

// LaunchTransform launches an external transform plugin binary and returns
// the transform.Plugin interface along with a cleanup function.
func LaunchTransform(binary string, opts ...Option) (transform.Plugin, func(), error) {
	o := applyOptions(opts)

	client, err := launchClient(binary, transform.PluginMap, o)
	if err != nil {
		return nil, nil, fmt.Errorf("launching transform plugin %s: %w", binary, err)
	}

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to transform plugin %s: %w", binary, err)
	}

	raw, err := rpcClient.Dispense("transform")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing transform plugin %s: %w", binary, err)
	}

	p, ok := raw.(transform.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("transform plugin %s: dispensed value does not implement transform.Plugin", binary)
	}

	return p, client.Kill, nil
}

// LaunchStore launches an external store plugin binary and returns the
// store.Plugin interface along with a cleanup function.
func LaunchStore(binary string, opts ...Option) (store.Plugin, func(), error) {
	o := applyOptions(opts)

	client, err := launchClient(binary, store.PluginMap, o)
	if err != nil {
		return nil, nil, fmt.Errorf("launching store plugin %s: %w", binary, err)
	}

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to store plugin %s: %w", binary, err)
	}

	raw, err := rpcClient.Dispense("store")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispensing store plugin %s: %w", binary, err)
	}

	p, ok := raw.(store.Plugin)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("store plugin %s: dispensed value does not implement store.Plugin", binary)
	}

	return p, client.Kill, nil
}

// launchClient validates the binary, audits it, and creates a go-plugin client.
func launchClient(binary string, pluginMap map[string]goplugin.Plugin, o *options) (*goplugin.Client, error) {
	log := o.log()

	resolved, err := validateBinaryPath(binary)
	if err != nil {
		return nil, err
	}

	if err := AuditBinary(log, resolved); err != nil {
		if o.auditMode == AuditHardFail {
			return nil, err
		}
		// AuditWarnOnly: error was already logged by AuditBinary.
	}

	cmd := exec.Command(resolved)

	// Isolate children in a separate process group so signals to the
	// parent don't propagate directly to children.
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Build isolated environment if requested.
	if o.isolatedEnv != nil {
		env := []string{
			"ZHI_PLUGIN=zhiplugin-v1",
			"PATH=" + os.Getenv("PATH"),
		}
		for k, v := range o.isolatedEnv {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          pluginMap,
		Cmd:              cmd,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	})

	return client, nil
}
