// Command zhi-marketplace runs the zhi marketplace server.
//
// The marketplace is a metadata and discovery service that indexes plugins
// published to OCI registries. It provides search, plugin detail, and
// download resolution endpoints.
//
// Usage:
//
//	zhi-marketplace --db marketplace.db --listen :8080
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MrWong99/zhi/cmd/zhi-marketplace/auth"
	"github.com/MrWong99/zhi/cmd/zhi-marketplace/server"
	"github.com/MrWong99/zhi/cmd/zhi-marketplace/storage"
)

func main() {
	var (
		dbPath      string
		listen      string
		registries  string
		apiKeys     string
		apiKeysFile string
	)

	flag.StringVar(&dbPath, "db", "marketplace.db", "SQLite database path")
	flag.StringVar(&listen, "listen", ":8080", "HTTP listen address")
	flag.StringVar(&registries, "oci-registries", "ghcr.io,docker.io", "comma-separated list of known OCI registries")
	flag.StringVar(&apiKeys, "api-keys", "", "comma-separated key=publisherID pairs for API authentication")
	flag.StringVar(&apiKeysFile, "api-keys-file", "", "path to file containing key=publisherID pairs (one per line); file is deleted after reading")
	flag.Parse()

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer store.Close()

	// Parse API keys from flag and/or file. File keys are merged with flag keys.
	keys := parseAPIKeys(apiKeys)
	if apiKeysFile != "" {
		fileKeys, err := readAPIKeysFile(apiKeysFile)
		if err != nil {
			log.Fatalf("reading api-keys-file: %v", err)
		}
		for k, v := range fileKeys {
			keys[k] = v
		}
	}
	ensurePublishers(store, keys)
	authenticator := auth.NewAuthenticator(keys)

	// Parse registries.
	var regList []string
	for r := range strings.SplitSeq(registries, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			regList = append(regList, r)
		}
	}

	handler := server.NewHandler(store, authenticator, regList)
	mux := server.NewMux(handler)

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("marketplace server listening on %s", listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown error: %v\n", err)
	}
}

// ensurePublishers creates publisher records for any API key publisher IDs
// that don't yet exist in the store. This handles first-run bootstrapping.
func ensurePublishers(store storage.Store, keys map[string]string) {
	seen := make(map[string]bool)
	for _, pubID := range keys {
		if seen[pubID] {
			continue
		}
		seen[pubID] = true
		if existing, err := store.GetPublisherByID(pubID); err == nil && existing != nil {
			continue
		}
		pub := &storage.Publisher{
			ID:   pubID,
			Name: pubID,
		}
		if err := store.CreatePublisher(pub); err != nil {
			log.Printf("warning: could not create publisher %q: %v", pubID, err)
		} else {
			log.Printf("created publisher %q from --api-keys", pubID)
		}
	}
}

// readAPIKeysFile reads key=publisherID pairs from a file (one per line) and
// deletes the file afterwards. Blank lines and lines starting with # are ignored.
func readAPIKeysFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("deleting %s after read: %w", path, err)
	}
	log.Printf("read and deleted api-keys file %s", path)

	keys := make(map[string]string)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			keys[parts[0]] = parts[1]
		}
	}
	return keys, nil
}

// parseAPIKeys parses "key1=pub1,key2=pub2" into a map.
func parseAPIKeys(s string) map[string]string {
	keys := make(map[string]string)
	if s == "" {
		return keys
	}
	for pair := range strings.SplitSeq(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) == 2 {
			keys[parts[0]] = parts[1]
		}
	}
	return keys
}
