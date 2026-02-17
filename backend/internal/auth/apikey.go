package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const (
	APIKeyPrefix = "moltgame_sk_"
	keyBytes     = 32 // 32 bytes = 64 hex chars
)

// GenerateAPIKey creates a new API key with the moltgame_sk_ prefix.
// Returns (plaintext key, SHA-256 hash of key).
func GenerateAPIKey() (string, string, error) {
	b := make([]byte, keyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	plainKey := APIKeyPrefix + hex.EncodeToString(b)
	hash := HashAPIKey(plainKey)
	return plainKey, hash, nil
}

// GenerateClaimToken creates a random claim token for Twitter verification.
func GenerateClaimToken() (string, error) {
	b := make([]byte, 40) // 40 bytes = 80 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate claim token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateVerificationCode creates a short code for Twitter post verification.
func GenerateVerificationCode() (string, error) {
	b := make([]byte, 4) // 4 bytes = 8 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}
	return "MOLT-" + hex.EncodeToString(b), nil
}
