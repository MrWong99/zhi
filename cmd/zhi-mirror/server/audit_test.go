package server

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditLoggerLog(t *testing.T) {
	var buf bytes.Buffer
	logger := NewAuditLogger(&buf)

	logger.Log(AuditEntry{
		Timestamp: "2026-02-05T10:30:00Z",
		Action:    "pull",
		Client:    "10.0.1.42",
		Artifact:  "zhi-project/zhi-config-ansible",
		Version:   "v1.2.0",
		Platform:  "linux/amd64",
		Digest:    "sha256:abc123",
		Cached:    true,
		User:      "alice@company.com",
	})

	output := buf.String()
	if output == "" {
		t.Fatal("expected output, got empty")
	}

	var entry AuditEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("parsing audit entry: %v", err)
	}

	if entry.Action != "pull" {
		t.Errorf("action = %q, want pull", entry.Action)
	}
	if entry.Client != "10.0.1.42" {
		t.Errorf("client = %q, want 10.0.1.42", entry.Client)
	}
	if entry.Artifact != "zhi-project/zhi-config-ansible" {
		t.Errorf("artifact = %q", entry.Artifact)
	}
	if !entry.Cached {
		t.Error("expected cached=true")
	}
	if entry.User != "alice@company.com" {
		t.Errorf("user = %q", entry.User)
	}
}

func TestAuditLoggerLogPull(t *testing.T) {
	var buf bytes.Buffer
	logger := NewAuditLogger(&buf)

	logger.LogPull("192.168.1.1", "myorg/plugin", "v2.0.0", "linux/arm64", "sha256:def456", false)

	var entry AuditEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("parsing audit entry: %v", err)
	}

	if entry.Action != "pull" {
		t.Errorf("action = %q, want pull", entry.Action)
	}
	if entry.Cached {
		t.Error("expected cached=false")
	}
	if entry.Timestamp == "" {
		t.Error("expected timestamp to be set automatically")
	}
}

func TestAuditLoggerMultipleEntries(t *testing.T) {
	var buf bytes.Buffer
	logger := NewAuditLogger(&buf)

	logger.LogPull("10.0.0.1", "a/b", "v1", "", "sha256:111", true)
	logger.LogPull("10.0.0.2", "c/d", "v2", "", "sha256:222", false)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}
}
