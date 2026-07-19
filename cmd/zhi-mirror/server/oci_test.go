package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MrWong99/zhi/cmd/zhi-mirror/storage"
)

func TestParseManifestPath(t *testing.T) {
	tests := []struct {
		path     string
		wantRepo string
		wantRef  string
	}{
		{
			path:     "/v2/zhi-project/zhi-config-ansible/manifests/v1.2.0",
			wantRepo: "zhi-project/zhi-config-ansible",
			wantRef:  "v1.2.0",
		},
		{
			path:     "/v2/zhi-project/zhi-config-ansible/manifests/sha256:abc123",
			wantRepo: "zhi-project/zhi-config-ansible",
			wantRef:  "sha256:abc123",
		},
		{
			path:     "/v2/simple/manifests/latest",
			wantRepo: "simple",
			wantRef:  "latest",
		},
		{
			path:     "/invalid",
			wantRepo: "",
			wantRef:  "",
		},
		{
			path:     "/v2/",
			wantRepo: "",
			wantRef:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			repo, ref := parseManifestPath(tt.path)
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if ref != tt.wantRef {
				t.Errorf("ref = %q, want %q", ref, tt.wantRef)
			}
		})
	}
}

func TestParseBlobPath(t *testing.T) {
	tests := []struct {
		path     string
		wantRepo string
		wantDgst string
	}{
		{
			path:     "/v2/zhi-project/plugin/blobs/sha256:abc123",
			wantRepo: "zhi-project/plugin",
			wantDgst: "sha256:abc123",
		},
		{
			path:     "/v2/simple/blobs/sha256:def456",
			wantRepo: "simple",
			wantDgst: "sha256:def456",
		},
		{
			path:     "/invalid",
			wantRepo: "",
			wantDgst: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			repo, dgst := parseBlobPath(tt.path)
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if dgst != tt.wantDgst {
				t.Errorf("dgst = %q, want %q", dgst, tt.wantDgst)
			}
		})
	}
}

func TestParseTagsListPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/v2/zhi-project/plugin/tags/list", "zhi-project/plugin"},
		{"/v2/simple/tags/list", "simple"},
		{"/v2/tags/list", ""},
		{"/invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := parseTagsListPath(tt.path)
			if got != tt.want {
				t.Errorf("parseTagsListPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "OCI manifest",
			data: `{"mediaType":"application/vnd.oci.image.manifest.v1+json"}`,
			want: "application/vnd.oci.image.manifest.v1+json",
		},
		{
			name: "OCI index",
			data: `{"mediaType":"application/vnd.oci.image.index.v1+json"}`,
			want: "application/vnd.oci.image.index.v1+json",
		},
		{
			name: "no mediaType field",
			data: `{"schemaVersion":2}`,
			want: "application/vnd.oci.image.manifest.v1+json",
		},
		{
			name: "invalid JSON",
			data: "not json",
			want: "application/vnd.oci.image.manifest.v1+json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMediaType([]byte(tt.data))
			if got != tt.want {
				t.Errorf("detectMediaType = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidRepo(t *testing.T) {
	tests := []struct {
		repo string
		want bool
	}{
		{"zhi-project/plugin", true},
		{"simple", true},
		{"a/b/c", true},
		{"", false},
		{"../etc", false},
		{"foo/../../etc/passwd", false},
		{"foo/./bar", false},
		{"foo//bar", false},
		{"/leading", false},
		{"trailing/", false},
	}
	for _, tt := range tests {
		if got := validRepo(tt.repo); got != tt.want {
			t.Errorf("validRepo(%q) = %v, want %v", tt.repo, got, tt.want)
		}
	}
}

// TestHandleBlobRejectsInvalidDigest is an F1 regression: a percent-decoded
// traversal digest in a blob request must be rejected with 400 before it
// reaches the storage layer, and no out-of-tree file may be served.
func TestHandleBlobRejectsInvalidDigest(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}
	h := NewOCIHandler(store, "upstream.invalid", DefaultPolicy(), NewAuditLogger(io.Discard), nil)

	// r.URL.Path carries the already-decoded traversal sequence, as Go's
	// ServeMux does not redirect the percent-encoded form.
	req := httptest.NewRequest(http.MethodGet, "http://mirror/v2/foo/blobs/x", nil)
	req.URL.Path = "/v2/foo/blobs/sha256:../../../../../../etc/passwd"
	rec := httptest.NewRecorder()

	h.HandleBlob(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleManifestRejectsInvalidDigestRef(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewOCILayout(dir)
	if err != nil {
		t.Fatalf("NewOCILayout: %v", err)
	}
	h := NewOCIHandler(store, "upstream.invalid", DefaultPolicy(), NewAuditLogger(io.Discard), nil)

	req := httptest.NewRequest(http.MethodGet, "http://mirror/v2/foo/manifests/x", nil)
	req.URL.Path = "/v2/foo/manifests/sha256:../../../../etc/passwd"
	rec := httptest.NewRecorder()

	h.HandleManifest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{
			name:       "remote addr with port",
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name:       "x-forwarded-for",
			remoteAddr: "127.0.0.1:12345",
			forwarded:  "10.0.1.42, 192.168.1.1",
			want:       "10.0.1.42",
		},
		{
			name:       "no port",
			remoteAddr: "192.168.1.1",
			want:       "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     http.Header{},
			}
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			got := clientIP(r)
			if got != tt.want {
				t.Errorf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}
