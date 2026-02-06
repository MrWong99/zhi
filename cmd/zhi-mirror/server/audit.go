package server

import (
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"
)

// AuditEntry represents a single audit log entry for a pull operation.
type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Client    string `json:"client"`
	Artifact  string `json:"artifact"`
	Version   string `json:"version,omitempty"`
	Platform  string `json:"platform,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Cached    bool   `json:"cached"`
	User      string `json:"user,omitempty"`
}

// AuditLogger writes structured JSON audit entries.
type AuditLogger struct {
	mu      sync.Mutex
	encoder *json.Encoder
	logger  *log.Logger
}

// NewAuditLogger creates an audit logger that writes JSON entries to the
// given writer (typically a file or os.Stdout).
func NewAuditLogger(w io.Writer) *AuditLogger {
	return &AuditLogger{
		encoder: json.NewEncoder(w),
		logger:  log.New(w, "", 0),
	}
}

// Log writes an audit entry.
func (a *AuditLogger) Log(entry AuditEntry) {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.encoder.Encode(entry); err != nil {
		log.Printf("audit: failed to write entry: %v", err)
	}
}

// LogPull logs a pull operation.
func (a *AuditLogger) LogPull(clientIP, artifact, version, platform, digest string, cached bool) {
	a.Log(AuditEntry{
		Action:   "pull",
		Client:   clientIP,
		Artifact: artifact,
		Version:  version,
		Platform: platform,
		Digest:   digest,
		Cached:   cached,
	})
}
