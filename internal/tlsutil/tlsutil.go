// Package tlsutil provides shared TLS configuration for zhi HTTP servers
// and clients. It supports server TLS, mutual TLS (mTLS) with client
// certificate verification, configurable minimum TLS version, and cipher
// suite selection. For outbound connections it provides [ClientConfig] to
// present client certificates and trust custom CAs.
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds TLS settings shared across zhi HTTP servers.
type Config struct {
	// CertFile is the path to the server certificate PEM file.
	CertFile string
	// KeyFile is the path to the server private key PEM file.
	KeyFile string
	// ClientCAFile is the path to a CA PEM file used to verify client
	// certificates. When set, mutual TLS is enabled and clients must
	// present a valid certificate signed by this CA.
	ClientCAFile string
	// MinVersion is the minimum TLS version to accept. Supported values
	// are "1.2" and "1.3". Defaults to "1.2" when empty.
	MinVersion string
	// CipherSuites is a list of allowed TLS cipher suite names. When
	// empty, the Go default cipher suites are used. Names must match
	// the values returned by [tls.CipherSuites] or
	// [tls.InsecureCipherSuites] (e.g. "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256").
	CipherSuites []string
}

// Enabled reports whether TLS is configured (both CertFile and KeyFile
// are set).
func (c *Config) Enabled() bool {
	return c.CertFile != "" && c.KeyFile != ""
}

// Validate checks that all configured file paths exist and are accessible.
// It is called automatically by [TLSConfig] but can also be called early
// at startup to catch configuration errors before they cause opaque
// failures deep in the TLS handshake.
func (c *Config) Validate() error {
	if c.CertFile != "" {
		if _, err := os.Stat(c.CertFile); err != nil {
			return fmt.Errorf("TLS certificate file not found at %q: %w", c.CertFile, err)
		}
	}
	if c.KeyFile != "" {
		if _, err := os.Stat(c.KeyFile); err != nil {
			return fmt.Errorf("TLS key file not found at %q: %w", c.KeyFile, err)
		}
	}
	if c.ClientCAFile != "" {
		if _, err := os.Stat(c.ClientCAFile); err != nil {
			return fmt.Errorf("client CA file not found at %q: %w", c.ClientCAFile, err)
		}
	}
	return nil
}

// TLSConfig builds a [tls.Config] from the configuration. It returns an
// error if the certificate cannot be loaded, the client CA cannot be
// parsed, an unknown cipher suite name is given, or the minimum version
// string is invalid.
//
// The caller should only call TLSConfig when [Enabled] returns true.
func (c *Config) TLSConfig() (*tls.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading TLS certificate: %w", err)
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Minimum TLS version.
	minVer := c.MinVersion
	if minVer == "" {
		minVer = "1.2"
	}
	switch minVer {
	case "1.2":
		cfg.MinVersion = tls.VersionTLS12
	case "1.3":
		cfg.MinVersion = tls.VersionTLS13
	default:
		return nil, fmt.Errorf("unsupported TLS minimum version %q (use \"1.2\" or \"1.3\")", minVer)
	}

	// Cipher suites.
	if len(c.CipherSuites) > 0 {
		suites, err := resolveCipherSuites(c.CipherSuites)
		if err != nil {
			return nil, err
		}
		cfg.CipherSuites = suites
	}

	// Mutual TLS.
	if c.ClientCAFile != "" {
		caPEM, err := os.ReadFile(c.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("reading client CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("client CA file %q contains no valid certificates", c.ClientCAFile)
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return cfg, nil
}

// RegisterFlags adds TLS-related flags to the given [flag.FlagSet].
func RegisterFlags(fs *flag.FlagSet, c *Config) {
	fs.StringVar(&c.CertFile, "tls-cert", "", "Path to TLS certificate PEM file")
	fs.StringVar(&c.KeyFile, "tls-key", "", "Path to TLS private key PEM file")
	fs.StringVar(&c.ClientCAFile, "tls-client-ca", "", "Path to CA PEM for client certificate verification (enables mTLS)")
	fs.StringVar(&c.MinVersion, "tls-min-version", "1.2", "Minimum TLS version (\"1.2\" or \"1.3\")")
	fs.Func("tls-cipher-suites", "Comma-separated list of allowed TLS cipher suites", func(s string) error {
		for name := range strings.SplitSeq(s, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				c.CipherSuites = append(c.CipherSuites, name)
			}
		}
		return nil
	})
}

// ClientConfig holds TLS settings for outbound HTTP connections. It
// allows the client to present a certificate (for mTLS) and/or trust a
// custom CA (e.g. a self-signed server certificate).
type ClientConfig struct {
	// CertFile is the path to the client certificate PEM file.
	CertFile string
	// KeyFile is the path to the client private key PEM file.
	KeyFile string
	// CAFile is the path to a CA PEM file to trust for server
	// verification. When empty, the system certificate pool is used.
	CAFile string
	// InsecureSkipVerify disables TLS certificate verification.
	// This should only be used for development and testing.
	InsecureSkipVerify bool
}

// Enabled reports whether any client TLS setting is configured.
func (c *ClientConfig) Enabled() bool {
	return (c.CertFile != "" && c.KeyFile != "") || c.CAFile != "" || c.InsecureSkipVerify
}

// Validate checks that all configured file paths exist and are accessible.
// It is called automatically by [HTTPClient] but can also be called early
// at startup to catch configuration errors before they cause opaque
// failures deep in the TLS handshake.
func (c *ClientConfig) Validate() error {
	if c.CAFile != "" {
		if _, err := os.Stat(c.CAFile); err != nil {
			return fmt.Errorf("CA certificate file not found at %q: %w", c.CAFile, err)
		}
	}
	if c.CertFile != "" {
		if _, err := os.Stat(c.CertFile); err != nil {
			return fmt.Errorf("client certificate file not found at %q: %w", c.CertFile, err)
		}
	}
	if c.KeyFile != "" {
		if _, err := os.Stat(c.KeyFile); err != nil {
			return fmt.Errorf("client key file not found at %q: %w", c.KeyFile, err)
		}
	}
	return nil
}

// HTTPClient returns an [http.Client] configured with the client TLS
// settings. If no settings are configured it returns nil.
func (c *ClientConfig) HTTPClient(timeout time.Duration) (*http.Client, error) {
	if !c.Enabled() {
		return nil, nil
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	tc, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: tc,
		},
	}, nil
}

// tlsConfig builds a [tls.Config] for client-side connections.
func (c *ClientConfig) tlsConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify, //nolint:gosec // user-requested via explicit opt-in
	}

	// Client certificate for mTLS.
	if c.CertFile != "" && c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	// Custom CA for server verification.
	if c.CAFile != "" {
		caPEM, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("CA file %q contains no valid certificates", c.CAFile)
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}

// resolveCipherSuites maps cipher suite names to their uint16 IDs.
func resolveCipherSuites(names []string) ([]uint16, error) {
	lookup := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		lookup[cs.Name] = cs.ID
	}
	for _, cs := range tls.InsecureCipherSuites() {
		lookup[cs.Name] = cs.ID
	}

	ids := make([]uint16, 0, len(names))
	for _, name := range names {
		id, ok := lookup[name]
		if !ok {
			return nil, fmt.Errorf("unknown TLS cipher suite %q", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
