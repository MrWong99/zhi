package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MrWong99/zhi/pkg/zhiplugin/config"
)

func TestRenderBuiltinJSON(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	result, err := Export(context.Background(), td, ExportRunConfig{
		Format: "json",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Export json: %v", err)
	}

	// Should be valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Content), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\nContent: %s", err, result.Content)
	}

	// Should contain enabled components.
	if _, ok := parsed["database"]; !ok {
		t.Error("JSON missing database")
	}
	if _, ok := parsed["app"]; !ok {
		t.Error("JSON missing app")
	}
	if _, ok := parsed["redis"]; !ok {
		t.Error("JSON missing redis")
	}

	// Should not contain disabled components.
	if _, ok := parsed["monitoring"]; ok {
		t.Error("JSON should not include monitoring (disabled)")
	}
}

func TestRenderBuiltinYAML(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	result, err := Export(context.Background(), td, ExportRunConfig{
		Format: "yaml",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Export yaml: %v", err)
	}

	if !strings.Contains(result.Content, "host: localhost") {
		t.Errorf("YAML missing host: %s", result.Content)
	}
	// Disabled component should not appear.
	if strings.Contains(result.Content, "scrape-interval") {
		t.Errorf("YAML should not include monitoring (disabled): %s", result.Content)
	}
}

func TestRenderBuiltinTOML(t *testing.T) {
	tree := config.NewTree()
	_ = tree.Set("server/host", &config.Value{Val: "localhost"})
	_ = tree.Set("server/port", &config.Value{Val: int64(8080)})
	td := NewTreeData(tree, nil)

	result, err := Export(context.Background(), td, ExportRunConfig{
		Format: "toml",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Export toml: %v", err)
	}

	if !strings.Contains(result.Content, "host") || !strings.Contains(result.Content, "localhost") {
		t.Errorf("TOML output missing host: %s", result.Content)
	}
}

func TestRenderBuiltinDotenv(t *testing.T) {
	tree := newTestTree()
	cm := newTestComponents(t)
	filtered := cm.FilterTree(tree)
	td := NewTreeData(filtered, cm)

	result, err := Export(context.Background(), td, ExportRunConfig{
		Format: "dotenv",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Export dotenv: %v", err)
	}

	if !strings.Contains(result.Content, "DATABASE_HOST=localhost") {
		t.Errorf("dotenv missing DATABASE_HOST: %s", result.Content)
	}
	if !strings.Contains(result.Content, "DATABASE_PORT=5432") {
		t.Errorf("dotenv missing DATABASE_PORT: %s", result.Content)
	}
	// Should not contain disabled components.
	if strings.Contains(result.Content, "MONITORING") {
		t.Errorf("dotenv should not include monitoring (disabled): %s", result.Content)
	}
}

func TestRenderBuiltinUnknownFormat(t *testing.T) {
	td := NewTreeData(config.NewTree(), nil)
	_, err := Export(context.Background(), td, ExportRunConfig{
		Format: "xml",
		DryRun: true,
	})
	if err == nil {
		t.Error("unknown format should return error")
	}
}

func TestRenderBuiltinJSONWithPrefix(t *testing.T) {
	tree := newTestTree()
	td := NewTreeData(tree, nil)

	result, err := Export(context.Background(), td, ExportRunConfig{
		Format: "json",
		Prefix: "database",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Export json with prefix: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Content), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\nContent: %s", err, result.Content)
	}

	if _, ok := parsed["host"]; !ok {
		t.Error("JSON missing host under database prefix")
	}
	if _, ok := parsed["port"]; !ok {
		t.Error("JSON missing port under database prefix")
	}
	// Should not have top-level app or redis keys.
	if _, ok := parsed["app"]; ok {
		t.Error("JSON should not include app when prefix is database")
	}
}
