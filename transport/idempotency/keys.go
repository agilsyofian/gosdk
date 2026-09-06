package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashBytes computes the SHA-256 hex digest of the given byte slice.
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// HashString computes the SHA-256 hex digest of the given string.
func HashString(s string) string {
	return HashBytes([]byte(s))
}

// BuildCanonicalKey joins a prefix and parts into a normalized canonical key.
func BuildCanonicalKey(prefix string, parts ...string) string {
	var sb strings.Builder
	if prefix != "" {
		sb.WriteString(prefix)
	}

	for _, part := range parts {
		if part == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString(":")
		}
		sb.WriteString(part)
	}

	return sb.String()
}
