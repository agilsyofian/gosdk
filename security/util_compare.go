package security

import "crypto/subtle"

// ConstantTimeEqual performs a constant-time comparison of two
// secrets (e.g. static API keys, HMAC digests). It lives in the core
// module, rather than in any single provider module, so every
// authenticator/session provider module can depend on it without
// depending on each other.
//
// Not suitable for comparing passwords against a password hash -- use
// a proper password hashing scheme (bcrypt/argon2/scrypt) for that
// instead.
func ConstantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		// Still perform a comparison to avoid a short-circuit timing
		// signal based on length, though length itself is not
		// typically considered sensitive for API keys.
		subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
