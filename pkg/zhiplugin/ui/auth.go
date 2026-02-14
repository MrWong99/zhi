package ui

// StoreAuthMethod describes an authentication method supported by the store.
type StoreAuthMethod struct {
	Type        string
	Description string
	Fields      []StoreAuthField
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
