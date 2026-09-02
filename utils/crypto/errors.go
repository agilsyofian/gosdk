// Package crypto implements the backend side of the frontend<->backend
// application-layer encryption protocol described in STEERFILE.md.
//
// Business/service packages MUST NOT depend on the types in this file for
// branching logic beyond the exported ErrorCode — the human-readable
// message is intentionally generic and MUST be safe to return to a client.
package crypto

import (
	"errors"
	"fmt"
)

// ErrorCode classifies a cryptographic failure without revealing
// sensitive internal detail. Callers may branch on the code; they MUST
// NOT surface Error.Internal() to end users.
type ErrorCode string

// Error codes returned by this package. Every code maps to the single
// generic external message "invalid encrypted payload" unless noted
// otherwise — see Error.Error().
const (
	// ErrCodeMalformedEnvelope indicates the envelope JSON/headers were
	// missing required fields or failed structural validation.
	ErrCodeMalformedEnvelope ErrorCode = "malformed_envelope"
	// ErrCodeUnsupportedVersion indicates the envelope declared a
	// protocol version this backend does not (or no longer) supports.
	ErrCodeUnsupportedVersion ErrorCode = "unsupported_version"
	// ErrCodeUnsupportedAlgorithm indicates the envelope declared an
	// algorithm identifier that is not the pinned, approved algorithm.
	ErrCodeUnsupportedAlgorithm ErrorCode = "unsupported_algorithm"
	// ErrCodeUnknownKeyID indicates key_id does not resolve via the
	// configured KeyProvider (never revealed to the caller in detail).
	ErrCodeUnknownKeyID ErrorCode = "unknown_key_id"
	// ErrCodeKeyUnwrapFailed indicates RSA-OAEP unwrapping of the AES
	// key failed.
	ErrCodeKeyUnwrapFailed ErrorCode = "key_unwrap_failed"
	// ErrCodeAuthenticationFailed indicates AES-GCM authentication
	// (tag verification) failed — the ciphertext, tag, IV, or AAD were
	// tampered with, mismatched, or corrupted.
	ErrCodeAuthenticationFailed ErrorCode = "authentication_failed"
	// ErrCodePayloadTooLarge indicates an attacker-controlled length
	// (envelope, encrypted key, ciphertext, or decrypted plaintext)
	// exceeded a configured bound before allocation/decryption.
	ErrCodePayloadTooLarge ErrorCode = "payload_too_large"
	// ErrCodeStaleTimestamp indicates the request timestamp fell
	// outside the configured freshness window (too old or too far in
	// the future).
	ErrCodeStaleTimestamp ErrorCode = "stale_timestamp"
	// ErrCodeReplayDetected indicates the request_id has already been
	// processed within the replay window.
	ErrCodeReplayDetected ErrorCode = "replay_detected"
	// ErrCodeKeyProviderUnavailable indicates the underlying KMS/HSM/
	// secret manager could not be reached.
	ErrCodeKeyProviderUnavailable ErrorCode = "key_provider_unavailable"

	// --- Field-level protocol (fields.go), per encryption-steerfile.md §22 ---
	// These map onto the same generic external message as every other
	// code above; they exist for internal observability granularity
	// only (STEERFILE.md §22: "SHOULD distinguish failures ... MUST
	// NOT expose detailed cryptographic internals to external
	// clients").

	// ErrCodeInvalidNonce indicates X-Nonce was missing, malformed, or
	// did not match the nonce embedded in an encrypted field value.
	ErrCodeInvalidNonce ErrorCode = "invalid_nonce"
	// ErrCodeInvalidTimestamp indicates X-Timestamp was missing,
	// non-numeric, or did not match the timestamp embedded in an
	// encrypted field value. Distinct from ErrCodeExpiredRequest,
	// which is a freshness-window judgement made by a ReplayProtector,
	// not by FieldDecryptor.
	ErrCodeInvalidTimestamp ErrorCode = "invalid_timestamp"
	// ErrCodeExpiredRequest indicates a ReplayProtector rejected a
	// timestamp as outside the configured freshness window.
	ErrCodeExpiredRequest ErrorCode = "expired_request"
	// ErrCodeReplayedRequest indicates a ReplayProtector rejected a
	// nonce it had already seen.
	ErrCodeReplayedRequest ErrorCode = "replayed_request"
	// ErrCodeInvalidEncryptedValue indicates a field's decoded binary
	// value did not match the expected IV||Ciphertext||Tag||Nonce||
	// Timestamp layout (e.g. too short).
	ErrCodeInvalidEncryptedValue ErrorCode = "invalid_encrypted_value"
)

// Error is the structured error type returned by this package. External()
// is safe to return to a client (e.g. as an HTTP 400 body). Internal()
// carries additional structured context for logs/tracing and MUST NEVER
// be returned to a client directly.
//
// Neither External() nor Internal() ever includes plaintext, keys, tags,
// or full ciphertext — see STEERFILE.md §18.
type Error struct {
	Code ErrorCode
	// internal is wrapped context for observability only (e.g. which
	// validation step failed, key_id involved, request_id). It MUST NOT
	// be populated with secrets or plaintext by any caller of newError.
	internal error
}

func newError(code ErrorCode, internalFmt string, args ...any) *Error {
	return &Error{Code: code, internal: fmt.Errorf(internalFmt, args...)}
}

// Error implements the standard error interface. It intentionally
// returns the same generic message for every code so that it is always
// safe to surface directly to an API client, per STEERFILE.md §17.
func (e *Error) Error() string {
	return "invalid encrypted payload"
}

// Internal returns a detailed, loggable description of the failure for
// observability. Never send this to a client.
func (e *Error) Internal() string {
	if e.internal == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %v", e.Code, e.internal)
}

// Unwrap allows errors.Is/errors.As to see the wrapped internal error.
func (e *Error) Unwrap() error { return e.internal }

// CodeOf extracts the ErrorCode from err if it (or something it wraps)
// is a *Error. ok is false for any other error, including nil.
func CodeOf(err error) (code ErrorCode, ok bool) {
	var ce *Error
	if errors.As(err, &ce) {
		return ce.Code, true
	}
	return "", false
}
