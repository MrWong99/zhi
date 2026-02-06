package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *JSONFileStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.json")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetPublisher(t *testing.T) {
	s := newTestStore(t)

	pub := &Publisher{
		ID:       "pub-1",
		Name:     "zhi-project",
		Email:    "team@zhi.dev",
		Verified: true,
	}
	if err := s.CreatePublisher(pub); err != nil {
		t.Fatalf("CreatePublisher: %v", err)
	}

	got, err := s.GetPublisher("zhi-project")
	if err != nil {
		t.Fatalf("GetPublisher: %v", err)
	}
	if got == nil {
		t.Fatal("expected publisher, got nil")
	}
	if got.Name != "zhi-project" {
		t.Errorf("Name = %q", got.Name)
	}
	if !got.Verified {
		t.Error("expected Verified=true")
	}
}

func TestGetPublisherByID(t *testing.T) {
	s := newTestStore(t)

	pub := &Publisher{ID: "pub-1", Name: "org", Email: "a@b.com"}
	if err := s.CreatePublisher(pub); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetPublisherByID("pub-1")
	if err != nil {
		t.Fatalf("GetPublisherByID: %v", err)
	}
	if got == nil || got.Name != "org" {
		t.Error("expected to find publisher by ID")
	}

	notFound, err := s.GetPublisherByID("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if notFound != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestDuplicatePublisher(t *testing.T) {
	s := newTestStore(t)

	pub := &Publisher{ID: "pub-1", Name: "org", Email: "a@b.com"}
	if err := s.CreatePublisher(pub); err != nil {
		t.Fatal(err)
	}
	pub2 := &Publisher{ID: "pub-2", Name: "org", Email: "b@b.com"}
	if err := s.CreatePublisher(pub2); err == nil {
		t.Error("expected error for duplicate publisher name")
	}
}

func TestCreateAndGetArtifact(t *testing.T) {
	s := newTestStore(t)

	pub := &Publisher{ID: "pub-1", Name: "zhi-project", Email: "a@b.com"}
	if err := s.CreatePublisher(pub); err != nil {
		t.Fatal(err)
	}

	art := &Artifact{
		ID:          "art-1",
		PublisherID: "pub-1",
		Name:        "ansible-config",
		Type:        "config",
		OCIRef:      "oci://ghcr.io/zhi-project/zhi-config-ansible",
		Description: "Ansible config provider",
		Keywords:    []string{"ansible", "config"},
	}
	if err := s.CreateArtifact(art); err != nil {
		t.Fatalf("CreateArtifact: %v", err)
	}

	got, err := s.GetArtifact("zhi-project", "ansible-config")
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if got == nil {
		t.Fatal("expected artifact, got nil")
	}
	if got.Name != "ansible-config" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.PublisherName != "zhi-project" {
		t.Errorf("PublisherName = %q", got.PublisherName)
	}
}

func TestCreateVersion(t *testing.T) {
	s := newTestStore(t)
	setupArtifact(t, s)

	v := &Version{
		ID:         "ver-1",
		ArtifactID: "art-1",
		Version:    "1.0.0",
		Digest:     "sha256:abc",
		Platforms:  []string{"linux/amd64"},
	}
	if err := s.CreateVersion(v); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	// Duplicate should fail.
	v2 := &Version{
		ID:         "ver-2",
		ArtifactID: "art-1",
		Version:    "1.0.0",
	}
	if err := s.CreateVersion(v2); err == nil {
		t.Error("expected error for duplicate version")
	}
}

func TestListVersions(t *testing.T) {
	s := newTestStore(t)
	setupArtifact(t, s)

	now := time.Now().UTC()
	v1 := &Version{ID: "v1", ArtifactID: "art-1", Version: "1.0.0", PublishedAt: now.Add(-time.Hour)}
	v2 := &Version{ID: "v2", ArtifactID: "art-1", Version: "1.1.0", PublishedAt: now}

	if err := s.CreateVersion(v1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVersion(v2); err != nil {
		t.Fatal(err)
	}

	versions, err := s.ListVersions("art-1")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len = %d, want 2", len(versions))
	}
	// Newest first.
	if versions[0].Version != "1.1.0" {
		t.Errorf("first version = %q, want 1.1.0", versions[0].Version)
	}
}

func TestGetVersion(t *testing.T) {
	s := newTestStore(t)
	setupArtifact(t, s)

	v := &Version{ID: "v1", ArtifactID: "art-1", Version: "1.0.0"}
	if err := s.CreateVersion(v); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetVersion("art-1", "1.0.0")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got == nil {
		t.Fatal("expected version")
	}
	if got.Version != "1.0.0" {
		t.Errorf("Version = %q", got.Version)
	}

	missing, err := s.GetVersion("art-1", "9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Error("expected nil for missing version")
	}
}

func TestSearchByQuery(t *testing.T) {
	s := newTestStore(t)

	pub := &Publisher{ID: "pub-1", Name: "org", Email: "a@b.com"}
	if err := s.CreatePublisher(pub); err != nil {
		t.Fatal(err)
	}

	art1 := &Artifact{ID: "a1", PublisherID: "pub-1", Name: "ansible-config", Type: "config", Description: "Ansible inventory", OCIRef: "oci://ref1", Keywords: []string{"ansible"}}
	art2 := &Artifact{ID: "a2", PublisherID: "pub-1", Name: "vault-store", Type: "store", Description: "HashiCorp Vault", OCIRef: "oci://ref2", Keywords: []string{"vault"}}

	if err := s.CreateArtifact(art1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateArtifact(art2); err != nil {
		t.Fatal(err)
	}

	// Search for "ansible".
	results, total, err := s.Search(SearchParams{Query: "ansible"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d", len(results))
	}
	if results[0].Artifact.Name != "ansible-config" {
		t.Errorf("Name = %q", results[0].Artifact.Name)
	}

	// Search with no query returns all.
	_, total, err = s.Search(SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
}

func TestSearchByType(t *testing.T) {
	s := newTestStore(t)

	pub := &Publisher{ID: "pub-1", Name: "org", Email: "a@b.com"}
	s.CreatePublisher(pub)

	art1 := &Artifact{ID: "a1", PublisherID: "pub-1", Name: "a", Type: "config", OCIRef: "r1"}
	art2 := &Artifact{ID: "a2", PublisherID: "pub-1", Name: "b", Type: "store", OCIRef: "r2"}
	s.CreateArtifact(art1)
	s.CreateArtifact(art2)

	results, _, err := s.Search(SearchParams{Type: "store"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d", len(results))
	}
	if results[0].Artifact.Name != "b" {
		t.Errorf("Name = %q", results[0].Artifact.Name)
	}
}

func TestSearchPagination(t *testing.T) {
	s := newTestStore(t)

	pub := &Publisher{ID: "pub-1", Name: "org", Email: "a@b.com"}
	s.CreatePublisher(pub)

	for i := 0; i < 5; i++ {
		art := &Artifact{
			ID:          s.nextID(),
			PublisherID: "pub-1",
			Name:        string(rune('a' + i)),
			Type:        "config",
			OCIRef:      "r",
		}
		s.CreateArtifact(art)
	}

	results, total, err := s.Search(SearchParams{PerPage: 2, Page: 1, Sort: "name"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(results) != 2 {
		t.Fatalf("page 1 len = %d, want 2", len(results))
	}

	results2, _, _ := s.Search(SearchParams{PerPage: 2, Page: 3, Sort: "name"})
	if len(results2) != 1 {
		t.Errorf("page 3 len = %d, want 1", len(results2))
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	// Create store and add data.
	s1, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	pub := &Publisher{ID: "p1", Name: "org", Email: "a@b.com"}
	s1.CreatePublisher(pub)
	s1.Close()

	// Reopen and verify data persisted.
	s2, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	got, err := s2.GetPublisher("org")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "org" {
		t.Error("expected publisher to persist across reopens")
	}
}

func setupArtifact(t *testing.T, s *JSONFileStore) {
	t.Helper()
	pub := &Publisher{ID: "pub-1", Name: "org", Email: "a@b.com"}
	if err := s.CreatePublisher(pub); err != nil {
		t.Fatal(err)
	}
	art := &Artifact{
		ID:          "art-1",
		PublisherID: "pub-1",
		Name:        "test-plugin",
		Type:        "config",
		OCIRef:      "oci://ref",
	}
	if err := s.CreateArtifact(art); err != nil {
		t.Fatal(err)
	}
}
