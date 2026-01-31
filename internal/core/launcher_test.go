package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// findProjectRoot walks up from the current directory to find the go.mod file.
func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// buildPlugin builds a Go plugin binary from the given source directory
// and returns the path to the built binary.
func buildPlugin(t *testing.T, srcDir, binaryName string) string {
	t.Helper()
	outDir := t.TempDir()
	out := filepath.Join(outDir, binaryName)
	cmd := exec.Command("go", "build", "-o", out, srcDir)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building plugin %s: %v\n%s", srcDir, err, output)
	}
	return out
}

func TestLaunchConfigPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping launcher test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	root := findProjectRoot(t)
	bin := buildPlugin(t, filepath.Join(root, "examples", "zhi-config-pokedex"), "zhi-config-pokedex")

	p, cleanup, err := LaunchConfig(bin)
	if err != nil {
		t.Fatalf("LaunchConfig: %v", err)
	}
	defer cleanup()

	// Test List().
	paths, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("List returned empty paths")
	}

	// Test Get().
	v, ok, err := p.Get(context.Background(), "pokedex/trainer.name")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: not found")
	}
	if v.Val != "Ash" {
		t.Errorf("Val = %v, want %q", v.Val, "Ash")
	}
}

func TestLaunchTransformPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping launcher test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	root := findProjectRoot(t)
	bin := buildPlugin(t, filepath.Join(root, "examples", "zhi-transform-pokedex"), "zhi-transform-pokedex")

	p, cleanup, err := LaunchTransform(bin)
	if err != nil {
		t.Fatalf("LaunchTransform: %v", err)
	}
	defer cleanup()

	// Test ValidatePolicy().
	policy, err := p.ValidatePolicy(context.Background())
	if err != nil {
		t.Fatalf("ValidatePolicy: %v", err)
	}
	// The zhi-transform-pokedex plugin returns ValidateBeforeTransform.
	if policy != 0 { // ValidateBeforeTransform = 0
		t.Errorf("ValidatePolicy = %v, want 0 (ValidateBeforeTransform)", policy)
	}
}

func TestLaunchStorePlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping launcher test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	root := findProjectRoot(t)
	bin := buildPlugin(t, filepath.Join(root, "examples", "zhi-store-memory"), "zhi-store-memory")

	p, cleanup, err := LaunchStore(bin)
	if err != nil {
		t.Fatalf("LaunchStore: %v", err)
	}
	defer cleanup()

	// Test ListTrees (should be empty initially).
	trees, err := p.ListTrees(context.Background())
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	if len(trees) != 0 {
		t.Errorf("ListTrees = %v, want empty", trees)
	}

	// Test SupportsVersioning.
	versioning, err := p.SupportsVersioning(context.Background())
	if err != nil {
		t.Fatalf("SupportsVersioning: %v", err)
	}
	if versioning {
		t.Error("zhi-store-memory should not support versioning")
	}
}

func TestLaunchConfigInvalidPath(t *testing.T) {
	_, _, err := LaunchConfig("/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestLaunchTransformInvalidPath(t *testing.T) {
	_, _, err := LaunchTransform("/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestLaunchStoreInvalidPath(t *testing.T) {
	_, _, err := LaunchStore("/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestCleanupKillsProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping launcher test in short mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	root := findProjectRoot(t)
	bin := buildPlugin(t, filepath.Join(root, "examples", "zhi-store-memory"), "zhi-store-memory")

	_, cleanup, err := LaunchStore(bin)
	if err != nil {
		t.Fatalf("LaunchStore: %v", err)
	}

	// Calling cleanup should not panic.
	cleanup()
	// Calling cleanup again should also not panic (Kill is idempotent).
	cleanup()
}
