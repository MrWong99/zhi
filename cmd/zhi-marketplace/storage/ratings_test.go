package storage

import (
	"testing"
)

func seedStore(t *testing.T, s *JSONFileStore) {
	t.Helper()
	pub := &Publisher{ID: "pub-1", Name: "org", Email: "a@b.com", Verified: true}
	if err := s.CreatePublisher(pub); err != nil {
		t.Fatal(err)
	}
	art := &Artifact{ID: "art-1", PublisherID: "pub-1", Name: "myplugin", Type: "config", OCIRef: "oci://example/plugin"}
	if err := s.CreateArtifact(art); err != nil {
		t.Fatal(err)
	}
	v := &Version{ID: "v-1", ArtifactID: "art-1", Version: "1.0.0", Digest: "sha256:abc"}
	if err := s.CreateVersion(v); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOrUpdateRating(t *testing.T) {
	s := newTestStore(t)
	seedStore(t, s)

	r := &Rating{ArtifactID: "art-1", UserID: "pub-1", UserName: "org", Score: 4, Comment: "Nice"}
	if err := s.CreateOrUpdateRating(r); err != nil {
		t.Fatal(err)
	}

	ratings, total, err := s.ListRatings("art-1", 1, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if ratings[0].Score != 4 {
		t.Errorf("score = %d, want 4", ratings[0].Score)
	}

	// Update the rating.
	r2 := &Rating{ArtifactID: "art-1", UserID: "pub-1", UserName: "org", Score: 5, Comment: "Updated"}
	if err := s.CreateOrUpdateRating(r2); err != nil {
		t.Fatal(err)
	}

	ratings, total, _ = s.ListRatings("art-1", 1, 20, "")
	if total != 1 {
		t.Errorf("total after update = %d, want 1", total)
	}
	if ratings[0].Score != 5 {
		t.Errorf("score after update = %d, want 5", ratings[0].Score)
	}
}

func TestRatingAggregation(t *testing.T) {
	s := newTestStore(t)
	seedStore(t, s)

	// No ratings yet.
	avg, count, dist, err := s.GetRatingAggregation("art-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if avg != 0 {
		t.Errorf("avg = %f, want 0", avg)
	}
	if len(dist) == 0 {
		t.Error("expected non-nil distribution even with no ratings")
	}

	// Add ratings.
	for i, score := range []int{5, 5, 4, 4, 3} {
		r := &Rating{ArtifactID: "art-1", UserID: "user-" + string(rune('a'+i)), Score: score}
		if err := s.CreateOrUpdateRating(r); err != nil {
			t.Fatal(err)
		}
	}

	avg, count, dist, err = s.GetRatingAggregation("art-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
	// Bayesian: (10*4.0 + 5+5+4+4+3) / (10+5) = (40+21)/15 = 4.066...
	if avg < 4.0 || avg > 4.1 {
		t.Errorf("avg = %f, expected ~4.07 (Bayesian weighted)", avg)
	}
	if dist[5] != 2 || dist[4] != 2 || dist[3] != 1 {
		t.Errorf("dist = %v", dist)
	}
}

func TestIncrementHelpful(t *testing.T) {
	s := newTestStore(t)
	seedStore(t, s)

	r := &Rating{ArtifactID: "art-1", UserID: "pub-1", UserName: "org", Score: 5}
	if err := s.CreateOrUpdateRating(r); err != nil {
		t.Fatal(err)
	}

	ratings, _, _ := s.ListRatings("art-1", 1, 20, "")
	ratingID := ratings[0].ID

	if err := s.IncrementHelpful(ratingID); err != nil {
		t.Fatal(err)
	}
	if err := s.IncrementHelpful(ratingID); err != nil {
		t.Fatal(err)
	}

	ratings, _, _ = s.ListRatings("art-1", 1, 20, "")
	if ratings[0].Helpful != 2 {
		t.Errorf("helpful = %d, want 2", ratings[0].Helpful)
	}
}

func TestIncrementHelpfulNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.IncrementHelpful("nonexistent"); err == nil {
		t.Error("expected error for nonexistent rating")
	}
}

func TestRecordDownload(t *testing.T) {
	s := newTestStore(t)
	seedStore(t, s)

	if err := s.RecordDownload("v-1", "linux/amd64"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordDownload("v-1", "darwin/arm64"); err != nil {
		t.Fatal(err)
	}

	total, monthly, err := s.GetDownloadStats("art-1")
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if monthly != 2 {
		t.Errorf("monthly = %d, want 2", monthly)
	}
}

func TestCreateAndListAdvisory(t *testing.T) {
	s := newTestStore(t)
	seedStore(t, s)

	adv := &Advisory{
		ArtifactID:       "art-1",
		PublisherName:    "org",
		PluginName:       "myplugin",
		Severity:         "high",
		Title:            "Test advisory",
		Description:      "A test advisory",
		AffectedVersions: "<1.0.0",
		FixedVersion:     "1.0.0",
	}
	if err := s.CreateAdvisory(adv); err != nil {
		t.Fatal(err)
	}

	advisories, err := s.ListAdvisories("", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(advisories) != 1 {
		t.Fatalf("len = %d, want 1", len(advisories))
	}
	if advisories[0].Title != "Test advisory" {
		t.Errorf("title = %q", advisories[0].Title)
	}

	// Filter by artifact.
	advisories, _ = s.ListAdvisories("art-1", "")
	if len(advisories) != 1 {
		t.Errorf("filter by artifact: len = %d", len(advisories))
	}

	// Filter by wrong artifact.
	advisories, _ = s.ListAdvisories("art-999", "")
	if len(advisories) != 0 {
		t.Errorf("filter by wrong artifact: len = %d", len(advisories))
	}

	// Filter by severity.
	advisories, _ = s.ListAdvisories("", "high")
	if len(advisories) != 1 {
		t.Errorf("filter by severity: len = %d", len(advisories))
	}
	advisories, _ = s.ListAdvisories("", "low")
	if len(advisories) != 0 {
		t.Errorf("filter by wrong severity: len = %d", len(advisories))
	}
}

func TestUpdatePublisher(t *testing.T) {
	s := newTestStore(t)
	seedStore(t, s)

	pub, _ := s.GetPublisher("org")
	pub.VerificationTier = 2
	if err := s.UpdatePublisher(pub); err != nil {
		t.Fatal(err)
	}

	updated, _ := s.GetPublisher("org")
	if updated.VerificationTier != 2 {
		t.Errorf("tier = %d, want 2", updated.VerificationTier)
	}
}

func TestListRatingsSortByHelpful(t *testing.T) {
	s := newTestStore(t)
	seedStore(t, s)

	// Add ratings with different helpful counts.
	r1 := &Rating{ArtifactID: "art-1", UserID: "user-a", UserName: "alice", Score: 3}
	if err := s.CreateOrUpdateRating(r1); err != nil {
		t.Fatal(err)
	}
	r2 := &Rating{ArtifactID: "art-1", UserID: "user-b", UserName: "bob", Score: 5}
	if err := s.CreateOrUpdateRating(r2); err != nil {
		t.Fatal(err)
	}

	// Mark r2 as helpful twice.
	ratings, _, _ := s.ListRatings("art-1", 1, 20, "")
	for _, r := range ratings {
		if r.UserName == "bob" {
			s.IncrementHelpful(r.ID)
			s.IncrementHelpful(r.ID)
		}
	}

	// Sort by helpful.
	ratings, _, _ = s.ListRatings("art-1", 1, 20, "helpful")
	if len(ratings) < 2 {
		t.Fatalf("len = %d", len(ratings))
	}
	if ratings[0].UserName != "bob" {
		t.Errorf("first rating user = %q, want bob (most helpful)", ratings[0].UserName)
	}
}
