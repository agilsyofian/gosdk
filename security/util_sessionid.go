package security

import (
	"crypto/rand"
	"encoding/base64"
)

// NewSessionID generates a cryptographically random, URL-safe session
// ID.
//
// This lives in the core module so every session provider module
// (session/redis, session/memory, and any future session/<provider>)
// shares one implementation instead of each rolling its own.
func NewSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
