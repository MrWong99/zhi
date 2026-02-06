// Package storage defines the database models and storage interface
// for the zhi marketplace server.
package storage

import "time"

// Publisher represents a registered publisher (user or organisation).
type Publisher struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"createdAt"`
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

	// CreateArtifact registers a new plugin or workspace.
	CreateArtifact(art *Artifact) error
	// GetArtifact retrieves an artifact by publisher and name.
	GetArtifact(publisher, name string) (*Artifact, error)
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

	// Close releases any resources held by the store.
	Close() error
}
