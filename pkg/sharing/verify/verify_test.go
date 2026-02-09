package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelNone, "none"},
		{LevelSigned, "signed"},
		{LevelVerifiedPublisher, "verified-publisher"},
		{LevelStrict, "strict"},
		{Level(99), "Level(99)"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", int(tt.level), got, tt.want)
		}
	}
}

func TestResultOK(t *testing.T) {
	r := &Result{Signed: false}
	if !r.OK() {
		t.Error("expected OK for nil error")
	}

	r = &Result{Error: os.ErrNotExist}
	if r.OK() {
		t.Error("expected not OK for non-nil error")
	}
}

func TestResultSummary(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{
			name:   "unsigned",
			result: Result{Signed: false},
			want:   "unsigned",
		},
		{
			name:   "signed",
			result: Result{Signed: true, SigningIdentity: "release@zhi.dev"},
			want:   "signed by release@zhi.dev",
		},
		{
			name:   "trusted",
			result: Result{Signed: true, SigningIdentity: "release@zhi.dev", TrustedPublisher: true},
			want:   "signed by release@zhi.dev (trusted publisher)",
		},
		{
			name:   "error",
			result: Result{Error: os.ErrPermission},
			want:   "verification failed: permission denied",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResultStatusIndicator(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{"unsigned", Result{Signed: false}, "unsigned"},
		{"signed", Result{Signed: true}, "signed"},
		{"verified", Result{Signed: true, TrustedPublisher: true}, "verified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.StatusIndicator(); got != tt.want {
				t.Errorf("StatusIndicator() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerifyArtifactSkipVerify(t *testing.T) {
	v := NewVerifier(nil)
	r := v.VerifyArtifact("oci://ghcr.io/org/plugin:v1.0", true)
	if r.Error != nil {
		t.Errorf("expected no error, got %v", r.Error)
	}
	if r.Signed {
		t.Error("expected unsigned when skip-verify")
	}
	if r.Level != LevelNone {
		t.Errorf("Level = %v, want LevelNone", r.Level)
	}
}

func TestVerifyArtifactDefault(t *testing.T) {
	v := NewVerifier(nil)
	r := v.VerifyArtifact("oci://ghcr.io/org/plugin:v1.0", false)
	if r.Error != nil {
		t.Errorf("expected no error, got %v", r.Error)
	}
	if r.Level != LevelSigned {
		t.Errorf("Level = %v, want LevelSigned", r.Level)
	}
}

func TestVerifyArtifactStrictMode(t *testing.T) {
	policy := &Policy{RequireSignatures: true}
	v := NewVerifier(policy)
	r := v.VerifyArtifact("oci://ghcr.io/org/plugin:v1.0", false)
	if r.Error == nil {
		t.Error("expected error in strict mode for unsigned artifact")
	}
	if r.Level != LevelStrict {
		t.Errorf("Level = %v, want LevelStrict", r.Level)
	}
}

func TestVerifyArtifactBlockedPlugin(t *testing.T) {
	policy := &Policy{
		BlockedPlugins: []string{"untrusted/bad-plugin"},
	}
	v := NewVerifier(policy)
	r := v.VerifyArtifact("oci://ghcr.io/untrusted/bad-plugin:v1.0", false)
	if r.Error == nil {
		t.Error("expected error for blocked plugin")
	}
}

func TestVerifyArtifactRegistryNotAllowed(t *testing.T) {
	policy := &Policy{
		AllowedRegistries: []string{"ghcr.io"},
	}
	v := NewVerifier(policy)

	// Allowed registry.
	r := v.VerifyArtifact("oci://ghcr.io/org/plugin:v1.0", false)
	if r.Error != nil {
		t.Errorf("expected no error for allowed registry, got %v", r.Error)
	}

	// Disallowed registry.
	r = v.VerifyArtifact("oci://docker.io/org/plugin:v1.0", false)
	if r.Error == nil {
		t.Error("expected error for disallowed registry")
	}
}

func TestVerifyBinaryDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testbinary")
	content := []byte("hello world binary content")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}

	// Compute the expected digest.
	expected, err := ComputeBinaryDigest(path)
	if err != nil {
		t.Fatalf("ComputeBinaryDigest: %v", err)
	}

	// Should pass with correct digest.
	if err := VerifyBinaryDigest(path, expected); err != nil {
		t.Errorf("VerifyBinaryDigest: unexpected error: %v", err)
	}

	// Should fail with wrong digest.
	if err := VerifyBinaryDigest(path, "sha256:0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Error("expected error for wrong digest")
	}

	// Should pass when no expected digest.
	if err := VerifyBinaryDigest(path, ""); err != nil {
		t.Errorf("expected no error for empty expected digest, got %v", err)
	}
}

func TestComputeBinaryDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := ComputeBinaryDigest(path)
	if err != nil {
		t.Fatalf("ComputeBinaryDigest: %v", err)
	}
	if d == "" {
		t.Error("expected non-empty digest")
	}
	if len(d) < 10 {
		t.Errorf("digest too short: %q", d)
	}
	// SHA-256 produces 64 hex chars.
	if !hasPrefix(d, "sha256:") {
		t.Errorf("expected sha256: prefix, got %q", d)
	}
}

func TestComputeBinaryDigestNotExist(t *testing.T) {
	_, err := ComputeBinaryDigest("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestExtractRegistryHost(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"oci://ghcr.io/org/plugin:v1.0", "ghcr.io"},
		{"ghcr.io/org/plugin:v1.0", "ghcr.io"},
		{"harbor.internal:5000/repo/plugin:v1.0", "harbor.internal:5000"},
		{"singlehost", "singlehost"},
	}
	for _, tt := range tests {
		if got := extractRegistryHost(tt.ref); got != tt.want {
			t.Errorf("extractRegistryHost(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestParseDigest(t *testing.T) {
	tests := []struct {
		name    string
		digest  string
		wantErr bool
	}{
		{
			name:    "valid sha256",
			digest:  "sha256:abc123def456",
			wantErr: false,
		},
		{
			name:    "invalid format no colon",
			digest:  "sha256abc123",
			wantErr: true,
		},
		{
			name:    "invalid hex",
			digest:  "sha256:notahex",
			wantErr: true,
		},
		{
			name:    "empty digest",
			digest:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDigest(tt.digest)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDigest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifySignatureOptionsDefaults(t *testing.T) {
	opts := VerifySignatureOptions{}
	if opts.RequireTimestamp {
		t.Error("expected RequireTimestamp to default to false")
	}
	if opts.RequireCTLog {
		t.Error("expected RequireCTLog to default to false")
	}
	if opts.RequireTLog {
		t.Error("expected RequireTLog to default to false")
	}
}
