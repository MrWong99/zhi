package launch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	log := o.log()

	log.Info("launching config plugin", "binary", binary)
	client, err := launchClient(binary, config.PluginMap, o)
	if err != nil {
		return nil, nil, fmt.Errorf("launching config plugin %s: %w", binary, err)
	}

	log.Debug("connecting to config plugin process", "binary", binary)
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to config plugin %s: %w", binary, err)
	}

	log.Debug("dispensing config plugin interface", "binary", binary)
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

	log.Info("config plugin ready", "binary", binary)
	return p, client.Kill, nil
}

// LaunchTransform launches an external transform plugin binary and returns
// the transform.Plugin interface along with a cleanup function.
func LaunchTransform(binary string, opts ...Option) (transform.Plugin, func(), error) {
	o := applyOptions(opts)
	log := o.log()

	log.Info("launching transform plugin", "binary", binary)
	client, err := launchClient(binary, transform.PluginMap, o)
	if err != nil {
		return nil, nil, fmt.Errorf("launching transform plugin %s: %w", binary, err)
	}

	log.Debug("connecting to transform plugin process", "binary", binary)
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to transform plugin %s: %w", binary, err)
	}

	log.Debug("dispensing transform plugin interface", "binary", binary)
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

	log.Info("transform plugin ready", "binary", binary)
	return p, client.Kill, nil
}

// LaunchStore launches an external store plugin binary and returns the
// store.Plugin interface along with a cleanup function.
func LaunchStore(binary string, opts ...Option) (store.Plugin, func(), error) {
	o := applyOptions(opts)
	log := o.log()

	log.Info("launching store plugin", "binary", binary)
	client, err := launchClient(binary, store.PluginMap, o)
	if err != nil {
		return nil, nil, fmt.Errorf("launching store plugin %s: %w", binary, err)
	}

	log.Debug("connecting to store plugin process", "binary", binary)
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("connecting to store plugin %s: %w", binary, err)
	}

	log.Debug("dispensing store plugin interface", "binary", binary)
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

	log.Info("store plugin ready", "binary", binary)
	return p, client.Kill, nil
}

// pluginLogName derives a short, human-readable logger name from a plugin
// binary path. For example, "/home/user/.zhi/plugins/zhi-store-memory"
// becomes "plugin.store.memory". Non-standard names fall back to the
// filename.
func pluginLogName(binary string) string {
	base := filepath.Base(binary)
	if strings.HasPrefix(base, "zhi-") {
		rest := base[len("zhi-"):]
		// Convert "store-memory" to "store.memory" etc.
		rest = strings.Replace(rest, "-", ".", 1)
		return "plugin." + rest
	}
	return "plugin." + base
}

// launchClient validates the binary, audits it, and creates a go-plugin client.
// The logger is passed to go-plugin's ClientConfig so that plugin stderr output
// is captured and forwarded with a per-plugin name prefix for clean separation.
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
		log.Debug("using isolated environment for plugin", "binary", resolved, "env_count", len(env))
	}

	// Create a named sub-logger for this plugin so its output is
	// neatly separated in the host's log stream. go-plugin uses this
	// logger to capture and forward the child process's stderr.
	pluginLogger := log.Named(pluginLogName(resolved))

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  zhiplugin.Handshake,
		Plugins:          pluginMap,
		Cmd:              cmd,
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           pluginLogger,
	})

	return client, nil
}
