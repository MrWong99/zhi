package ui

// StoreAuthMethod describes an authentication method supported by the store.
type StoreAuthMethod struct {
	Type        string
	Description string
	Fields      []StoreAuthField
	Interactive bool // true if this method requires browser interaction (e.g. OIDC)
}

// StoreInteractiveChallenge holds the data needed to initiate a browser-based
// authentication flow.
type StoreInteractiveChallenge struct {
	ChallengeID string // opaque identifier for this auth attempt
	AuthURL     string // URL the user must visit in their browser
	ExpiresAt   string // RFC3339 expiry timestamp
}

// StoreAuthField describes a single credential field for an auth method.
type StoreAuthField struct {
	Name        string
	Description string
	Required    bool
	Secret      bool
}

// StoreSessionStatus represents the current authentication state as seen
// by the UI layer.
type StoreSessionStatus string

const (
	StoreSessionNone            StoreSessionStatus = "none"
	StoreSessionUnauthenticated StoreSessionStatus = "unauthenticated"
	StoreSessionAuthenticated   StoreSessionStatus = "authenticated"
	StoreSessionExpired         StoreSessionStatus = "expired"
)

// StoreSession holds the authentication state exposed to UI plugins.
type StoreSession struct {
	SessionID string
	Status    StoreSessionStatus
	ExpiresAt string            // RFC3339 or empty
	Metadata  map[string]string // informational only
}
