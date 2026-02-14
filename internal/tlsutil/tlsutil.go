// Package tlsutil provides shared TLS configuration for zhi HTTP servers.
// It supports server TLS, mutual TLS (mTLS) with client certificate
// verification, configurable minimum TLS version, and cipher suite
// selection.
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"strings"
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

// TLSConfig builds a [tls.Config] from the configuration. It returns an
// error if the certificate cannot be loaded, the client CA cannot be
// parsed, an unknown cipher suite name is given, or the minimum version
// string is invalid.
//
// The caller should only call TLSConfig when [Enabled] returns true.
func (c *Config) TLSConfig() (*tls.Config, error) {
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
