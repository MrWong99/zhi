// Package storage defines the database models and storage interface
// for the zhi marketplace server.
package storage

import "time"

// Publisher represents a registered publisher (user or organisation).
type Publisher struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Email            string    `json:"email"`
	Verified         bool      `json:"verified"`
	VerificationTier int       `json:"verificationTier"`
	CreatedAt        time.Time `json:"createdAt"`
}

// Rating represents a user's rating of an artifact.
type Rating struct {
	ID         string    `json:"id"`
	ArtifactID string    `json:"artifactId"`
	UserID     string    `json:"userId"`
	UserName   string    `json:"userName"`
	Score      int       `json:"score"`
	Comment    string    `json:"comment,omitempty"`
	Helpful    int       `json:"helpful"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Advisory represents a security vulnerability advisory for a plugin.
type Advisory struct {
	ID               string    `json:"id"`
	ArtifactID       string    `json:"artifactId"`
	PublisherName    string    `json:"publisherName"`
	PluginName       string    `json:"pluginName"`
	Severity         string    `json:"severity"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	AffectedVersions string    `json:"affectedVersions"`
	FixedVersion     string    `json:"fixedVersion,omitempty"`
	CVE              string    `json:"cve,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

// DownloadEvent records a single download for analytics.
type DownloadEvent struct {
	ID           string    `json:"id"`
	VersionID    string    `json:"versionId"`
	Platform     string    `json:"platform"`
	DownloadedAt time.Time `json:"downloadedAt"`
}

// Artifact represents a registered plugin or workspace.
type Artifact struct {
	ID              string    `json:"id"`
	PublisherID     string    `json:"publisherId"`
	PublisherName   string    `json:"publisherName"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	OCIRef          string    `json:"ociRef"`
	Description     string    `json:"description"`
	LongDescription string    `json:"longDescription,omitempty"`
	License         string    `json:"license,omitempty"`
	Homepage        string    `json:"homepage,omitempty"`
	Repository      string    `json:"repository,omitempty"`
	Keywords        []string  `json:"keywords,omitempty"`
	Verified        bool      `json:"verified"`
	Deprecated      bool      `json:"deprecated"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// Version represents a published version of an artifact.
type Version struct {
	ID                 string    `json:"id"`
	ArtifactID         string    `json:"artifactId"`
	Version            string    `json:"version"`
	Digest             string    `json:"digest"`
	Platforms          []string  `json:"platforms"`
	ZhiProtocolVersion string    `json:"zhiProtocolVersion,omitempty"`
	MinZhiVersion      string    `json:"minZhiVersion,omitempty"`
	Changelog          string    `json:"changelog,omitempty"`
	Signed             bool      `json:"signed"`
	SigningIdentity    string    `json:"signingIdentity,omitempty"`
	Downloads          int64     `json:"downloads"`
	PublishedAt        time.Time `json:"publishedAt"`
	Yanked             bool      `json:"yanked"`
}

// SearchParams holds the parameters for a search query.
type SearchParams struct {
	Query    string
	Type     string
	Sort     string
	Verified bool
	Page     int
	PerPage  int
}

// SearchResult is a single search hit with aggregate data.
type SearchResult struct {
	Artifact       Artifact `json:"artifact"`
	LatestVersion  string   `json:"latestVersion"`
	TotalDownloads int64    `json:"totalDownloads"`
	Rating         float64  `json:"rating"`
	RatingCount    int      `json:"ratingCount"`
	Platforms      []string `json:"platforms"`
}

// Store is the interface for marketplace data persistence.
type Store interface {
	// CreatePublisher registers a new publisher.
	CreatePublisher(pub *Publisher) error
	// GetPublisher retrieves a publisher by name.
	GetPublisher(name string) (*Publisher, error)
	// GetPublisherByID retrieves a publisher by ID.
	GetPublisherByID(id string) (*Publisher, error)
	// UpdatePublisher updates publisher fields.
	UpdatePublisher(pub *Publisher) error

	// CreateArtifact registers a new plugin or workspace.
	CreateArtifact(art *Artifact) error
	// GetArtifact retrieves an artifact by publisher, name, and type.
	// If pluginType is empty, the first matching artifact is returned
	// (for backward compatibility with routes that don't include type).
	GetArtifact(publisher, name, pluginType string) (*Artifact, error)
	// UpdateArtifact updates an existing artifact.
	UpdateArtifact(art *Artifact) error
	// Search performs full-text search across artifacts.
	Search(params SearchParams) ([]SearchResult, int, error)

	// CreateVersion adds a version to an artifact.
	CreateVersion(v *Version) error
	// ListVersions returns all versions for an artifact, newest first.
	ListVersions(artifactID string) ([]Version, error)
	// GetVersion retrieves a specific version.
	GetVersion(artifactID, version string) (*Version, error)

	// CreateOrUpdateRating submits or updates a rating for an artifact.
	CreateOrUpdateRating(r *Rating) error
	// ListRatings lists ratings for an artifact with pagination.
	ListRatings(artifactID string, page, perPage int, sort string) ([]Rating, int, error)
	// IncrementHelpful increments the helpful count for a rating.
	IncrementHelpful(ratingID string) error
	// GetRatingAggregation returns the average score, count, and distribution for an artifact.
	GetRatingAggregation(artifactID string) (avg float64, count int, distribution map[int]int, err error)

	// RecordDownload records a download event for a version.
	RecordDownload(versionID, platform string) error
	// GetDownloadStats returns total and monthly download counts for an artifact.
	GetDownloadStats(artifactID string) (total, monthly int64, err error)

	// CreateAdvisory publishes a security advisory.
	CreateAdvisory(a *Advisory) error
	// ListAdvisories lists advisories, optionally filtered by artifact or severity.
	ListAdvisories(artifactID, severity string) ([]Advisory, error)

	// Close releases any resources held by the store.
	Close() error
}
