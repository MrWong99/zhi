package launch

import (
	"log/slog"
	"os"
	"testing"
)

func TestWithLogger(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
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
