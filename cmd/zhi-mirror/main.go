// Command zhi-mirror runs a local registry mirror for enterprise and
// air-gapped environments. It acts as a caching proxy for OCI registries
// and the marketplace API, with policy controls and audit logging.
//
// Usage:
//
//	zhi-mirror serve --listen :5050 --storage /var/lib/zhi-mirror --upstream-registry ghcr.io
//	zhi-mirror export --artifacts publisher/name:v1.0.0 --output bundle.tar
//	zhi-mirror import --input bundle.tar
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/MrWong99/zhi/cmd/zhi-mirror/airgap"
	"github.com/MrWong99/zhi/cmd/zhi-mirror/server"
	"github.com/MrWong99/zhi/cmd/zhi-mirror/storage"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		if err := runServe(os.Args[2:]); err != nil {
			log.Fatalf("serve: %v", err)
		}
	case "export":
		if err := runExport(os.Args[2:]); err != nil {
			log.Fatalf("export: %v", err)
		}
	case "import":
		if err := runImport(os.Args[2:]); err != nil {
			log.Fatalf("import: %v", err)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `zhi-mirror - Local registry mirror for enterprise and air-gapped environments

Commands:
  serve     Start the mirror server
  export    Export artifacts to a bundle for air-gapped transfer
  import    Import a bundle into the mirror storage

Run 'zhi-mirror <command> --help' for command-specific usage.
`)
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		listen              string
		storageDir          string
		upstreamRegistry    string
		upstreamMarketplace string
		policyFile          string
		auditFile           string
	)
	fs.StringVar(&listen, "listen", ":5050", "HTTP listen address")
	fs.StringVar(&storageDir, "storage", defaultStorageDir(), "Storage directory")
	fs.StringVar(&upstreamRegistry, "upstream-registry", "ghcr.io", "Upstream OCI registry")
	fs.StringVar(&upstreamMarketplace, "upstream-marketplace", "https://marketplace.zhi.dev", "Upstream marketplace URL")
	fs.StringVar(&policyFile, "policy", server.DefaultPolicyPath(), "Policy YAML file path")
	fs.StringVar(&auditFile, "audit", "", "Audit log file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load policy.
	policy, err := server.LoadPolicy(policyFile)
	if err != nil {
		return fmt.Errorf("loading policy: %w", err)
	}

	// Set up audit logging.
	var auditWriter *os.File
	if auditFile != "" {
		if err := os.MkdirAll(filepath.Dir(auditFile), 0o755); err != nil {
			return fmt.Errorf("creating audit directory: %w", err)
		}
		auditWriter, err = os.OpenFile(auditFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("opening audit file: %w", err)
		}
		defer auditWriter.Close()
	} else {
		auditWriter = os.Stdout
	}
	audit := server.NewAuditLogger(auditWriter)

	// Set up storage.
	ociDir := filepath.Join(storageDir, "oci")
	ociStore, err := storage.NewOCILayout(ociDir)
	if err != nil {
		return fmt.Errorf("initializing OCI storage: %w", err)
	}

	metaDir := filepath.Join(storageDir, "metadata")
	metaStore, err := storage.NewMetadataStore(metaDir)
	if err != nil {
		return fmt.Errorf("initializing metadata store: %w", err)
	}

	// Create handlers.
	ociHandler := server.NewOCIHandler(ociStore, upstreamRegistry, policy, audit)
	marketplaceProxy := server.NewMarketplaceProxy(upstreamMarketplace, metaStore, policy)

	mux := server.NewMux(ociHandler, marketplaceProxy)

	srv := &http.Server{
		Addr:              listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start sync scheduler if configured.
	if len(policy.Sync) > 0 {
		scheduler := server.NewSyncScheduler(ociStore, upstreamRegistry, policy.Sync)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		scheduler.Start(ctx)
		defer scheduler.Stop()
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("zhi-mirror listening on %s (upstream registry: %s)", listen, upstreamRegistry)
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

	return nil
}

func runExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	var (
		artifacts    string
		output       string
		registry     string
		allPlatforms bool
	)
	fs.StringVar(&artifacts, "artifacts", "", "Comma-separated artifact references")
	fs.StringVar(&output, "output", "", "Output tarball path")
	fs.StringVar(&registry, "upstream-registry", "ghcr.io", "Upstream OCI registry")
	fs.BoolVar(&allPlatforms, "all-platforms", false, "Export all platform variants")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if artifacts == "" {
		return fmt.Errorf("--artifacts is required")
	}
	if output == "" {
		return fmt.Errorf("--output is required")
	}

	var refs []string
	for _, a := range strings.Split(artifacts, ",") {
		a = strings.TrimSpace(a)
		if a != "" {
			refs = append(refs, a)
		}
	}

	return airgap.Export(airgap.ExportOptions{
		Artifacts:        refs,
		AllPlatforms:     allPlatforms,
		Output:           output,
		UpstreamRegistry: registry,
	})
}

func runImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	var (
		input      string
		storageDir string
	)
	fs.StringVar(&input, "input", "", "Input bundle tarball path")
	fs.StringVar(&storageDir, "storage", defaultStorageDir(), "Storage directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if input == "" {
		return fmt.Errorf("--input is required")
	}

	ociDir := filepath.Join(storageDir, "oci")
	store, err := storage.NewOCILayout(ociDir)
	if err != nil {
		return fmt.Errorf("initializing OCI storage: %w", err)
	}

	manifest, err := airgap.Import(store, airgap.ImportOptions{Input: input})
	if err != nil {
		return err
	}

	log.Printf("imported %d artifacts from bundle", len(manifest.Artifacts))
	for _, a := range manifest.Artifacts {
		log.Printf("  %s (%s)", a.Ref, a.Digest)
	}
	return nil
}

// defaultStorageDir returns the default storage directory.
func defaultStorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/var/lib/zhi-mirror"
	}
	return filepath.Join(home, ".zhi", "mirror")
}
