package launch

import (
	"testing"

	"github.com/hashicorp/go-hclog"
)

func TestWithLogger(t *testing.T) {
	t.Parallel()
	logger := hclog.New(&hclog.LoggerOptions{Name: "test"})
	o := applyOptions([]Option{WithLogger(logger)})
	if o.logger != logger {
		t.Fatal("WithLogger did not set the logger")
	}
	if o.log() != logger {
		t.Fatal("log() did not return the configured logger")
	}
}

func TestWithLoggerDefault(t *testing.T) {
	t.Parallel()
	o := applyOptions(nil)
	if o.log() == nil {
		t.Fatal("log() returned nil without configured logger")
	}
}

func TestWithIsolatedEnv(t *testing.T) {
	t.Parallel()
	env := map[string]string{"FOO": "bar"}
	o := applyOptions([]Option{WithIsolatedEnv(env)})
	if o.isolatedEnv == nil {
		t.Fatal("WithIsolatedEnv did not set the environment")
	}
	if o.isolatedEnv["FOO"] != "bar" {
		t.Fatal("WithIsolatedEnv did not preserve values")
	}
}

func TestLaunchConfigInvalidBinary(t *testing.T) {
	t.Parallel()
	_, _, err := LaunchConfig("/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestLaunchTransformInvalidBinary(t *testing.T) {
	t.Parallel()
	_, _, err := LaunchTransform("/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestLaunchStoreInvalidBinary(t *testing.T) {
	t.Parallel()
	_, _, err := LaunchStore("/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestWithPluginOptions(t *testing.T) {
	t.Parallel()
	opts := map[string]any{"addr": "localhost:9090", "debug": true}
	o := applyOptions([]Option{WithPluginOptions(opts)})
	if o.pluginOptions == nil {
		t.Fatal("WithPluginOptions did not set options")
	}
	if o.pluginOptions["addr"] != "localhost:9090" {
		t.Errorf("addr = %v, want localhost:9090", o.pluginOptions["addr"])
	}
	if o.pluginOptions["debug"] != true {
		t.Errorf("debug = %v, want true", o.pluginOptions["debug"])
	}
}

func TestWithPluginOptionsNil(t *testing.T) {
	t.Parallel()
	o := applyOptions([]Option{WithPluginOptions(nil)})
	if o.pluginOptions != nil {
		t.Errorf("pluginOptions = %v, want nil", o.pluginOptions)
	}
}

func TestPluginLogName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		binary string
		want   string
	}{
		{"/home/user/.zhi/plugins/zhi-store-memory", "plugin.store.memory"},
		{"/home/user/.zhi/plugins/zhi-config-ansible", "plugin.config.ansible"},
		{"zhi-ui-mcp-sse", "plugin.ui.mcp-sse"},
		{"other-binary", "plugin.other-binary"},
	}
	for _, tt := range tests {
		got := pluginLogName(tt.binary)
		if got != tt.want {
			t.Errorf("pluginLogName(%q) = %q, want %q", tt.binary, got, tt.want)
		}
	}
}
