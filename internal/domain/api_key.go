package domain

import "time"

type APIKeyMode string

const (
	APIKeyModeLive APIKeyMode = "live"
	APIKeyModeTest APIKeyMode = "test"
)

type APIKey struct {
	ID         string
	Name       string
	KeyHash    string
	Mode       APIKeyMode
	IsRevoked  bool
	LastUsedAt *time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
}
