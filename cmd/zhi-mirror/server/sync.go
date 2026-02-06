package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MrWong99/zhi/cmd/zhi-mirror/storage"
)

// SyncScheduler pre-populates the mirror cache by fetching artifacts from
// upstream registries on a schedule.
type SyncScheduler struct {
	store    *storage.OCILayout
	upstream string
	rules    []SyncRule
	client   *http.Client
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewSyncScheduler creates a sync scheduler for the given rules.
func NewSyncScheduler(store *storage.OCILayout, upstream string, rules []SyncRule) *SyncScheduler {
	return &SyncScheduler{
		store:    store,
		upstream: strings.TrimRight(upstream, "/"),
		rules:    rules,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// Start begins the sync scheduler. It runs an initial sync immediately,
// then schedules periodic syncs based on the configured intervals.
func (s *SyncScheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	// Run initial sync.
	s.runSync(ctx)

	// Schedule periodic syncs. For simplicity, we parse the cron schedule
	// into a fixed interval. Full cron parsing would require an external
	// dependency.
	for _, rule := range s.rules {
		interval := parseSyncInterval(rule.Schedule)
		s.wg.Go(func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.syncRule(ctx, rule)
				}
			}
		})
	}
}

// Stop gracefully stops the scheduler.
func (s *SyncScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// runSync executes all sync rules once.
func (s *SyncScheduler) runSync(ctx context.Context) {
	for _, rule := range s.rules {
		s.syncRule(ctx, rule)
	}
}

// syncRule fetches a single artifact from the upstream registry and caches it.
func (s *SyncScheduler) syncRule(ctx context.Context, rule SyncRule) {
	ref := rule.Ref
	ref = strings.TrimPrefix(ref, "oci://")

	// Parse the reference to extract registry, repo, and tag.
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) < 2 {
		log.Printf("sync: invalid reference %q", rule.Ref)
		return
	}
	repoAndTag := parts[1]

	// Split repo and tag pattern.
	repo := repoAndTag
	tag := "latest"
	if idx := strings.LastIndex(repoAndTag, ":"); idx >= 0 {
		repo = repoAndTag[:idx]
		tag = repoAndTag[idx+1:]
	}

	// If tag contains a wildcard, we cannot enumerate — just log it.
	if strings.Contains(tag, "*") {
		log.Printf("sync: wildcard tags (%s) require tag listing; syncing latest for %s", tag, repo)
		tag = "latest"
	}

	log.Printf("sync: fetching %s:%s from %s", repo, tag, s.upstream)

	// Fetch manifest.
	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", s.upstream, repo, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		log.Printf("sync: failed to create request: %v", err)
		return
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("sync: failed to fetch manifest for %s:%s: %v", repo, tag, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("sync: upstream returned %d for %s:%s", resp.StatusCode, repo, tag)
		return
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("sync: failed to read manifest: %v", err)
		return
	}

	dgst, err := s.store.PutBlob(data)
	if err != nil {
		log.Printf("sync: failed to store manifest: %v", err)
		return
	}

	if err := s.store.PutTag(repo, tag, dgst); err != nil {
		log.Printf("sync: failed to store tag: %v", err)
		return
	}

	log.Printf("sync: cached %s:%s (%s)", repo, tag, dgst)
}

// parseSyncInterval converts a simplified cron-like schedule to a duration.
// Supports:
//   - "0 2 * * *"  → 24h (daily)
//   - "0 */6 * * *" → 6h (every 6 hours)
//   - "*/30 * * * *" → 30m (every 30 minutes)
//
// Falls back to 24h for unrecognised patterns.
func parseSyncInterval(schedule string) time.Duration {
	parts := strings.Fields(schedule)
	if len(parts) < 5 {
		return 24 * time.Hour
	}

	// Check for hour interval patterns like "0 */6 * * *".
	if hours, ok := strings.CutPrefix(parts[1], "*/"); ok {
		var h int
		if _, err := fmt.Sscanf(hours, "%d", &h); err == nil && h > 0 {
			return time.Duration(h) * time.Hour
		}
	}

	// Check for minute interval patterns like "*/30 * * * *".
	if mins, ok := strings.CutPrefix(parts[0], "*/"); ok {
		var m int
		if _, err := fmt.Sscanf(mins, "%d", &m); err == nil && m > 0 {
			return time.Duration(m) * time.Minute
		}
	}

	// Default: daily.
	return 24 * time.Hour
}
