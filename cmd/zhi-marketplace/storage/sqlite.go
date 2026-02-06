package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// idCounter provides unique IDs.
var idCounter atomic.Int64

// JSONFileStore implements Store using a JSON file for persistence.
// It loads the full dataset into memory and flushes to disk on writes.
// For production use with large datasets, this would be replaced with
// a proper SQLite or PostgreSQL backend.
type JSONFileStore struct {
	mu         sync.RWMutex
	path       string
	publishers map[string]*Publisher // id -> Publisher
	artifacts  map[string]*Artifact  // id -> Artifact
	versions   map[string][]*Version // artifactID -> []Version
	ratings    map[string][]*Rating  // artifactID -> []Rating
	advisories []*Advisory
	downloads  []*DownloadEvent
}

// storeData is the on-disk JSON format.
type storeData struct {
	Publishers []*Publisher     `json:"publishers"`
	Artifacts  []*Artifact      `json:"artifacts"`
	Versions   []*Version       `json:"versions"`
	Ratings    []*Rating        `json:"ratings,omitempty"`
	Advisories []*Advisory      `json:"advisories,omitempty"`
	Downloads  []*DownloadEvent `json:"downloads,omitempty"`
}

// NewSQLiteStore creates a JSONFileStore backed by a JSON file at the
// given path. The function name is retained for interface compatibility
// with configurations that reference it. If the path ends with ".db",
// the suffix is replaced with ".json".
func NewSQLiteStore(dsn string) (*JSONFileStore, error) {
	path := dsn
	if strings.HasSuffix(path, ".db") {
		path = strings.TrimSuffix(path, ".db") + ".json"
	}

	s := &JSONFileStore{
		path:       path,
		publishers: make(map[string]*Publisher),
		artifacts:  make(map[string]*Artifact),
		versions:   make(map[string][]*Version),
		ratings:    make(map[string][]*Rating),
	}

	// Load existing data if present.
	data, err := os.ReadFile(path)
	if err == nil {
		var sd storeData
		if json.Unmarshal(data, &sd) == nil {
			for _, p := range sd.Publishers {
				s.publishers[p.ID] = p
			}
			for _, a := range sd.Artifacts {
				s.artifacts[a.ID] = a
			}
			for _, v := range sd.Versions {
				s.versions[v.ArtifactID] = append(s.versions[v.ArtifactID], v)
			}
			for _, r := range sd.Ratings {
				s.ratings[r.ArtifactID] = append(s.ratings[r.ArtifactID], r)
			}
			s.advisories = sd.Advisories
			s.downloads = sd.Downloads
		}
	}

	return s, nil
}

func (s *JSONFileStore) flush() error {
	var sd storeData
	for _, p := range s.publishers {
		sd.Publishers = append(sd.Publishers, p)
	}
	for _, a := range s.artifacts {
		sd.Artifacts = append(sd.Artifacts, a)
	}
	for _, vv := range s.versions {
		sd.Versions = append(sd.Versions, vv...)
	}
	for _, rr := range s.ratings {
		sd.Ratings = append(sd.Ratings, rr...)
	}
	sd.Advisories = s.advisories
	sd.Downloads = s.downloads

	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func (s *JSONFileStore) nextID() string {
	return fmt.Sprintf("id-%d-%d", time.Now().UnixNano(), idCounter.Add(1))
}

func (s *JSONFileStore) CreatePublisher(pub *Publisher) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pub.ID == "" {
		pub.ID = s.nextID()
	}
	if pub.CreatedAt.IsZero() {
		pub.CreatedAt = time.Now().UTC()
	}

	for _, existing := range s.publishers {
		if existing.Name == pub.Name {
			return fmt.Errorf("publisher %q already exists", pub.Name)
		}
	}

	s.publishers[pub.ID] = pub
	return s.flush()
}

func (s *JSONFileStore) GetPublisher(name string) (*Publisher, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.publishers {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, nil
}

func (s *JSONFileStore) GetPublisherByID(id string) (*Publisher, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.publishers[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (s *JSONFileStore) CreateArtifact(art *Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if art.ID == "" {
		art.ID = s.nextID()
	}
	if art.CreatedAt.IsZero() {
		art.CreatedAt = time.Now().UTC()
	}
	art.UpdatedAt = art.CreatedAt

	for _, existing := range s.artifacts {
		if existing.PublisherID == art.PublisherID && existing.Name == art.Name {
			return fmt.Errorf("artifact %q already exists for this publisher", art.Name)
		}
	}

	if pub, ok := s.publishers[art.PublisherID]; ok {
		art.PublisherName = pub.Name
	}

	s.artifacts[art.ID] = art
	return s.flush()
}

func (s *JSONFileStore) GetArtifact(publisher, name string) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pubID string
	for _, p := range s.publishers {
		if p.Name == publisher {
			pubID = p.ID
			break
		}
	}
	if pubID == "" {
		return nil, nil
	}

	for _, a := range s.artifacts {
		if a.PublisherID == pubID && a.Name == name {
			return a, nil
		}
	}
	return nil, nil
}

func (s *JSONFileStore) UpdateArtifact(art *Artifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	art.UpdatedAt = time.Now().UTC()
	s.artifacts[art.ID] = art
	return s.flush()
}

func (s *JSONFileStore) Search(params SearchParams) ([]SearchResult, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if params.PerPage <= 0 {
		params.PerPage = 20
	}
	if params.Page <= 0 {
		params.Page = 1
	}

	var matches []SearchResult
	query := strings.ToLower(params.Query)

	for _, art := range s.artifacts {
		if art.Deprecated {
			continue
		}

		if params.Type != "" && art.Type != params.Type {
			continue
		}
		if params.Verified && !art.Verified {
			continue
		}

		if query != "" {
			matched := strings.Contains(strings.ToLower(art.Name), query) ||
				strings.Contains(strings.ToLower(art.Description), query)
			if !matched {
				for _, kw := range art.Keywords {
					if strings.Contains(strings.ToLower(kw), query) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		sr := SearchResult{Artifact: *art}

		versions := s.versions[art.ID]
		if len(versions) > 0 {
			sorted := make([]*Version, len(versions))
			copy(sorted, versions)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
			})
			sr.LatestVersion = sorted[0].Version
			sr.Platforms = sorted[0].Platforms
			for _, v := range sorted {
				sr.TotalDownloads += v.Downloads
			}
		}

		// Populate rating aggregation from ratings store.
		if rr := s.ratings[art.ID]; len(rr) > 0 {
			var sum float64
			for _, r := range rr {
				sum += float64(r.Score)
			}
			sr.RatingCount = len(rr)
			sr.Rating = (bayesianC*bayesianM + sum) / (bayesianC + float64(sr.RatingCount))
		}

		matches = append(matches, sr)
	}

	switch params.Sort {
	case "downloads":
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].TotalDownloads > matches[j].TotalDownloads
		})
	case "name":
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Artifact.Name < matches[j].Artifact.Name
		})
	default:
		sort.Slice(matches, func(i, j int) bool {
			return matches[i].Artifact.UpdatedAt.After(matches[j].Artifact.UpdatedAt)
		})
	}

	total := len(matches)
	offset := (params.Page - 1) * params.PerPage
	if offset >= len(matches) {
		return nil, total, nil
	}
	end := offset + params.PerPage
	if end > len(matches) {
		end = len(matches)
	}

	return matches[offset:end], total, nil
}

func (s *JSONFileStore) CreateVersion(v *Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v.ID == "" {
		v.ID = s.nextID()
	}
	if v.PublishedAt.IsZero() {
		v.PublishedAt = time.Now().UTC()
	}

	for _, existing := range s.versions[v.ArtifactID] {
		if existing.Version == v.Version {
			return fmt.Errorf("version %q already exists", v.Version)
		}
	}

	s.versions[v.ArtifactID] = append(s.versions[v.ArtifactID], v)

	if art, ok := s.artifacts[v.ArtifactID]; ok {
		art.UpdatedAt = time.Now().UTC()
	}

	return s.flush()
}

func (s *JSONFileStore) ListVersions(artifactID string) ([]Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vv := s.versions[artifactID]
	result := make([]Version, len(vv))
	for i, v := range vv {
		result[i] = *v
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].PublishedAt.After(result[j].PublishedAt)
	})

	return result, nil
}

func (s *JSONFileStore) GetVersion(artifactID, version string) (*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, v := range s.versions[artifactID] {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, nil
}

func (s *JSONFileStore) UpdatePublisher(pub *Publisher) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.publishers[pub.ID] = pub
	return s.flush()
}

func (s *JSONFileStore) CreateOrUpdateRating(r *Rating) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if r.ID == "" {
		r.ID = s.nextID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	// Check for existing rating by same user on same artifact.
	ratings := s.ratings[r.ArtifactID]
	for i, existing := range ratings {
		if existing.UserID == r.UserID {
			r.CreatedAt = existing.CreatedAt
			r.Helpful = existing.Helpful
			ratings[i] = r
			return s.flush()
		}
	}

	s.ratings[r.ArtifactID] = append(s.ratings[r.ArtifactID], r)
	return s.flush()
}

func (s *JSONFileStore) ListRatings(artifactID string, page, perPage int, sortBy string) ([]Rating, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}

	rr := s.ratings[artifactID]
	result := make([]Rating, len(rr))
	for i, r := range rr {
		result[i] = *r
	}

	switch sortBy {
	case "helpful":
		sort.Slice(result, func(i, j int) bool {
			return result[i].Helpful > result[j].Helpful
		})
	default:
		sort.Slice(result, func(i, j int) bool {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		})
	}

	total := len(result)
	offset := (page - 1) * perPage
	if offset >= total {
		return nil, total, nil
	}
	end := offset + perPage
	if end > total {
		end = total
	}

	return result[offset:end], total, nil
}

func (s *JSONFileStore) IncrementHelpful(ratingID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rr := range s.ratings {
		for _, r := range rr {
			if r.ID == ratingID {
				r.Helpful++
				return s.flush()
			}
		}
	}
	return fmt.Errorf("rating %q not found", ratingID)
}

// bayesianAverage computes a Bayesian weighted average.
// C is the confidence threshold, m is the global prior mean.
const bayesianC = 10
const bayesianM = 4.0

func (s *JSONFileStore) GetRatingAggregation(artifactID string) (float64, int, map[int]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rr := s.ratings[artifactID]
	count := len(rr)
	dist := map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}

	if count == 0 {
		return 0, 0, dist, nil
	}

	var sum float64
	for _, r := range rr {
		sum += float64(r.Score)
		dist[r.Score]++
	}

	avg := (bayesianC*bayesianM + sum) / (bayesianC + float64(count))
	return avg, count, dist, nil
}

func (s *JSONFileStore) RecordDownload(versionID, platform string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	evt := &DownloadEvent{
		ID:           s.nextID(),
		VersionID:    versionID,
		Platform:     platform,
		DownloadedAt: time.Now().UTC(),
	}
	s.downloads = append(s.downloads, evt)

	// Also increment version download counter.
	for _, vv := range s.versions {
		for _, v := range vv {
			if v.ID == versionID {
				v.Downloads++
				break
			}
		}
	}

	return s.flush()
}

func (s *JSONFileStore) GetDownloadStats(artifactID string) (int64, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect version IDs for this artifact.
	versionIDs := make(map[string]bool)
	for _, v := range s.versions[artifactID] {
		versionIDs[v.ID] = true
	}

	var total int64
	var monthly int64
	monthAgo := time.Now().UTC().AddDate(0, -1, 0)

	for _, d := range s.downloads {
		if versionIDs[d.VersionID] {
			total++
			if d.DownloadedAt.After(monthAgo) {
				monthly++
			}
		}
	}

	return total, monthly, nil
}

func (s *JSONFileStore) CreateAdvisory(a *Advisory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if a.ID == "" {
		a.ID = s.nextID()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}

	s.advisories = append(s.advisories, a)
	return s.flush()
}

func (s *JSONFileStore) ListAdvisories(artifactID, severity string) ([]Advisory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Advisory
	for _, a := range s.advisories {
		if artifactID != "" && a.ArtifactID != artifactID {
			continue
		}
		if severity != "" && a.Severity != severity {
			continue
		}
		result = append(result, *a)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (s *JSONFileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}
