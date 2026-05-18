package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashAPIKey returns a deterministic hash for API key lookup/storage.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
