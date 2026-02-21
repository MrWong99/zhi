package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCert creates a self-signed certificate and key in dir,
// returning the file paths.
func generateTestCert(t *testing.T, dir, prefix string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certFile = filepath.Join(dir, prefix+"cert.pem")
	keyFile = filepath.Join(dir, prefix+"key.pem")

	cf, err := os.Create(certFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatal(err)
	}
	cf.Close()

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	kf, err := os.Create(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatal(err)
	}
	kf.Close()

	return certFile, keyFile
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cert     string
		key      string
		expected bool
	}{
		{"both set", "cert.pem", "key.pem", true},
		{"cert only", "cert.pem", "", false},
		{"key only", "", "key.pem", false},
		{"neither", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{CertFile: tt.cert, KeyFile: tt.key}
			if got := c.Enabled(); got != tt.expected {
				t.Errorf("Enabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTLSConfig_BasicCert(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "server-")

	c := &Config{CertFile: certFile, KeyFile: keyFile}
	cfg, err := c.TLSConfig()
	if err != nil {
		t.Fatalf("TLSConfig() error: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(cfg.Certificates))
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("default MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS12)
	}
	if cfg.ClientAuth != tls.NoClientCert {
		t.Errorf("ClientAuth = %d, want NoClientCert", cfg.ClientAuth)
	}
}

func TestTLSConfig_MinVersion(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "")

	tests := []struct {
		version string
		want    uint16
		wantErr bool
	}{
		{"1.2", tls.VersionTLS12, false},
		{"1.3", tls.VersionTLS13, false},
		{"", tls.VersionTLS12, false}, // default
		{"1.1", 0, true},
		{"1.0", 0, true},
		{"bogus", 0, true},
	}
	for _, tt := range tests {
		t.Run("version="+tt.version, func(t *testing.T) {
			c := &Config{CertFile: certFile, KeyFile: keyFile, MinVersion: tt.version}
			cfg, err := c.TLSConfig()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.MinVersion != tt.want {
				t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tt.want)
			}
		})
	}
}

func TestTLSConfig_MutualTLS(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "server-")
	caFile, _ := generateTestCert(t, dir, "ca-")

	c := &Config{
		CertFile:     certFile,
		KeyFile:      keyFile,
		ClientCAFile: caFile,
	}
	cfg, err := c.TLSConfig()
	if err != nil {
		t.Fatalf("TLSConfig() error: %v", err)
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("ClientAuth = %d, want RequireAndVerifyClientCert", cfg.ClientAuth)
	}
	if cfg.ClientCAs == nil {
		t.Fatal("ClientCAs is nil")
	}
}

func TestTLSConfig_MutualTLS_BadCA(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "")

	// Write a file that is not a valid PEM certificate.
	badCA := filepath.Join(dir, "bad-ca.pem")
	os.WriteFile(badCA, []byte("not a certificate"), 0o644)

	c := &Config{CertFile: certFile, KeyFile: keyFile, ClientCAFile: badCA}
	_, err := c.TLSConfig()
	if err == nil {
		t.Fatal("expected error for invalid CA file")
	}
}

func TestTLSConfig_MutualTLS_MissingCA(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "")

	c := &Config{CertFile: certFile, KeyFile: keyFile, ClientCAFile: filepath.Join(dir, "nonexistent.pem")}
	_, err := c.TLSConfig()
	if err == nil {
		t.Fatal("expected error for missing CA file")
	}
}

func TestTLSConfig_CipherSuites(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "")

	// Pick a known cipher suite.
	knownSuite := tls.CipherSuites()[0]

	c := &Config{
		CertFile:     certFile,
		KeyFile:      keyFile,
		CipherSuites: []string{knownSuite.Name},
	}
	cfg, err := c.TLSConfig()
	if err != nil {
		t.Fatalf("TLSConfig() error: %v", err)
	}
	if len(cfg.CipherSuites) != 1 || cfg.CipherSuites[0] != knownSuite.ID {
		t.Errorf("CipherSuites = %v, want [%d]", cfg.CipherSuites, knownSuite.ID)
	}
}

func TestTLSConfig_CipherSuites_Unknown(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "")

	c := &Config{
		CertFile:     certFile,
		KeyFile:      keyFile,
		CipherSuites: []string{"TLS_BOGUS_SUITE"},
	}
	_, err := c.TLSConfig()
	if err == nil {
		t.Fatal("expected error for unknown cipher suite")
	}
}

func TestTLSConfig_BadCertPath(t *testing.T) {
	c := &Config{CertFile: "/nonexistent/cert.pem", KeyFile: "/nonexistent/key.pem"}
	_, err := c.TLSConfig()
	if err == nil {
		t.Fatal("expected error for missing cert files")
	}
}

func TestRegisterFlags(t *testing.T) {
	var c Config
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	RegisterFlags(fs, &c)

	err := fs.Parse([]string{
		"--tls-cert", "server.crt",
		"--tls-key", "server.key",
		"--tls-client-ca", "ca.crt",
		"--tls-min-version", "1.3",
		"--tls-cipher-suites", "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384",
	})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if c.CertFile != "server.crt" {
		t.Errorf("CertFile = %q, want %q", c.CertFile, "server.crt")
	}
	if c.KeyFile != "server.key" {
		t.Errorf("KeyFile = %q, want %q", c.KeyFile, "server.key")
	}
	if c.ClientCAFile != "ca.crt" {
		t.Errorf("ClientCAFile = %q, want %q", c.ClientCAFile, "ca.crt")
	}
	if c.MinVersion != "1.3" {
		t.Errorf("MinVersion = %q, want %q", c.MinVersion, "1.3")
	}
	if len(c.CipherSuites) != 2 {
		t.Fatalf("CipherSuites length = %d, want 2", len(c.CipherSuites))
	}
	if c.CipherSuites[0] != "TLS_AES_128_GCM_SHA256" {
		t.Errorf("CipherSuites[0] = %q", c.CipherSuites[0])
	}
	if c.CipherSuites[1] != "TLS_AES_256_GCM_SHA384" {
		t.Errorf("CipherSuites[1] = %q", c.CipherSuites[1])
	}
}

// --- ClientConfig tests ---

func TestClientConfig_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ClientConfig
		expected bool
	}{
		{"empty", ClientConfig{}, false},
		{"cert+key", ClientConfig{CertFile: "c.pem", KeyFile: "k.pem"}, true},
		{"ca only", ClientConfig{CAFile: "ca.pem"}, true},
		{"all", ClientConfig{CertFile: "c.pem", KeyFile: "k.pem", CAFile: "ca.pem"}, true},
		{"cert only", ClientConfig{CertFile: "c.pem"}, false},
		{"key only", ClientConfig{KeyFile: "k.pem"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Enabled(); got != tt.expected {
				t.Errorf("Enabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClientConfig_HTTPClient_Disabled(t *testing.T) {
	c := &ClientConfig{}
	hc, err := c.HTTPClient(30 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc != nil {
		t.Fatal("expected nil client when not enabled")
	}
}

func TestClientConfig_HTTPClient_WithCert(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateTestCert(t, dir, "client-")

	c := &ClientConfig{CertFile: certFile, KeyFile: keyFile}
	hc, err := c.HTTPClient(30 * time.Second)
	if err != nil {
		t.Fatalf("HTTPClient() error: %v", err)
	}
	if hc == nil {
		t.Fatal("expected non-nil client")
	}
	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if len(tr.TLSClientConfig.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(tr.TLSClientConfig.Certificates))
	}
}

func TestClientConfig_HTTPClient_WithCA(t *testing.T) {
	dir := t.TempDir()
	caFile, _ := generateTestCert(t, dir, "ca-")

	c := &ClientConfig{CAFile: caFile}
	hc, err := c.HTTPClient(30 * time.Second)
	if err != nil {
		t.Fatalf("HTTPClient() error: %v", err)
	}
	if hc == nil {
		t.Fatal("expected non-nil client")
	}
	tr := hc.Transport.(*http.Transport)
	if tr.TLSClientConfig.RootCAs == nil {
		t.Fatal("expected RootCAs to be set")
	}
}

func TestClientConfig_HTTPClient_BadCert(t *testing.T) {
	c := &ClientConfig{CertFile: "/nonexistent/c.pem", KeyFile: "/nonexistent/k.pem"}
	_, err := c.HTTPClient(30 * time.Second)
	if err == nil {
		t.Fatal("expected error for missing cert files")
	}
}

func TestClientConfig_Enabled_InsecureSkipVerify(t *testing.T) {
	c := &ClientConfig{InsecureSkipVerify: true}
	if !c.Enabled() {
		t.Error("Enabled() should be true when InsecureSkipVerify is set")
	}
}

func TestClientConfig_HTTPClient_InsecureSkipVerify(t *testing.T) {
	c := &ClientConfig{InsecureSkipVerify: true}
	hc, err := c.HTTPClient(30 * time.Second)
	if err != nil {
		t.Fatalf("HTTPClient() error: %v", err)
	}
	if hc == nil {
		t.Fatal("expected non-nil client")
	}
	tr, ok := hc.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true in TLS config")
	}
}

func TestClientConfig_HTTPClient_BadCA(t *testing.T) {
	dir := t.TempDir()
	badCA := filepath.Join(dir, "bad.pem")
	os.WriteFile(badCA, []byte("not a cert"), 0o644)

	c := &ClientConfig{CAFile: badCA}
	_, err := c.HTTPClient(30 * time.Second)
	if err == nil {
		t.Fatal("expected error for invalid CA")
	}
}
