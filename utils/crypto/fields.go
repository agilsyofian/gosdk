package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"strconv"
)

// ---------------------------------------------------------------------------
// Implements the field-level protocol described in
// encryption-steerfile.md, version 1:
//
//   - Only the fields named by the caller are touched; the JSON
//     request schema is otherwise unchanged (§6, §25).
//   - One AES-256 key per HTTP request, wrapped once with
//     RSA-OAEP-SHA256 and carried in X-Encryption-Key (§7, §13).
//   - One nonce and one timestamp per HTTP request, carried in
//     X-Nonce / X-Timestamp AND embedded in every encrypted field
//     value so the two can be cross-checked (§10, §11, §12).
//   - Each field's value is
//       Base64URL(IV(12) || Ciphertext || Tag(16) || Nonce(16) || Timestamp(8, big-endian))
//     (§10).
//   - Replay protection (nonce-store uniqueness + timestamp freshness
//     policy) is a SEPARATE responsibility — see replay_protector.go —
//     and is never called from inside this file (§16, §17). What this
//     file DOES check is that the nonce/timestamp embedded in each
//     field's ciphertext match the ones declared in the request
//     headers, which is a structural/authenticity check on the
//     encrypted value itself, not a replay-store lookup.
//
// Note on what AES-GCM actually authenticates: per §9/§10, the
// Authentication Tag covers IV+Ciphertext only — the Nonce and
// Timestamp are appended alongside, outside the AEAD, and are verified
// here only by byte/integer equality against the headers. If a future
// protocol version needs the nonce/timestamp to be cryptographically
// bound (not just equality-checked), fold them into AES-GCM Additional
// Authenticated Data and bump X-Encryption-Version — don't change
// version "1"'s behavior in place (§23).
// ---------------------------------------------------------------------------

// Field encryption header names, per encryption-steerfile.md §12.
const (
	HeaderEncryptionVersion = "X-Encryption-Version"
	HeaderEncryptionKid     = "X-Encryption-Kid"
	HeaderEncryptionKey     = "X-Encryption-Key"
	HeaderNonce             = "X-Nonce"
	HeaderTimestamp         = "X-Timestamp"
)

// FieldEncryptionVersion1 is the only version this FieldDecryptor
// accepts.
const FieldEncryptionVersion1 = "1"

// Per-field binary layout sizes, per encryption-steerfile.md §10.
const (
	fieldNonceSize     = 16 // 128-bit request-level replay-protection nonce
	fieldTimestampSize = 8  // unix seconds, unsigned 64-bit big-endian
	// fieldFixedOverhead is every non-ciphertext byte in a field value:
	// IV(12) + Tag(16) + Nonce(16) + Timestamp(8).
	fieldFixedOverhead = aesGCMNonceSize + aesGCMTagSize + fieldNonceSize + fieldTimestampSize
)

// FieldHeaders mirrors the five encryption metadata headers defined in
// encryption-steerfile.md §12.
type FieldHeaders struct {
	Version      string // X-Encryption-Version
	KeyID        string // X-Encryption-Kid
	EncryptedKey string // X-Encryption-Key: base64url(RSA-OAEP-SHA256(AES-256 key))
	Nonce        string // X-Nonce: base64url(16-byte request nonce)
	Timestamp    string // X-Timestamp: unix seconds, decimal string
}

// FieldHeadersFromLookup builds FieldHeaders from any single-header
// accessor (e.g. (net/http).Header.Get, or a framework context's header
// getter), keeping this package independent of any particular HTTP
// framework — framework glue belongs in transport/http, per
// AI_AGENT.md §4.
func FieldHeadersFromLookup(get func(string) string) FieldHeaders {
	return FieldHeaders{
		Version:      get(HeaderEncryptionVersion),
		KeyID:        get(HeaderEncryptionKid),
		EncryptedKey: get(HeaderEncryptionKey),
		Nonce:        get(HeaderNonce),
		Timestamp:    get(HeaderTimestamp),
	}
}

// FieldDecryptor decrypts individual JSON string fields encrypted per
// encryption-steerfile.md. One FieldDecryptor is reused across
// requests; it is safe for concurrent use. It has no knowledge of HTTP,
// Redis, or replay policy — see the package doc comment above and
// replay_protector.go (§17's "Cryptographic Separation of
// Responsibility").
type FieldDecryptor struct {
	keys                KeyProvider
	maxEncryptedKeyB64  int
	maxFieldValueBase64 int
}

// FieldDecryptorOption configures a FieldDecryptor built by
// NewFieldDecryptor.
type FieldDecryptorOption func(*FieldDecryptor)

// WithMaxFieldValueBytes overrides the maximum accepted length (in
// Base64URL characters) of a single encrypted field value, bounding
// allocation before decoding, per §21 (input validation) in spirit.
func WithMaxFieldValueBytes(n int) FieldDecryptorOption {
	return func(d *FieldDecryptor) { d.maxFieldValueBase64 = n }
}

// NewFieldDecryptor builds a FieldDecryptor backed by keys — typically
// a *JWKSKeyProvider loaded per §5, but any KeyProvider works.
func NewFieldDecryptor(keys KeyProvider, opts ...FieldDecryptorOption) *FieldDecryptor {
	d := &FieldDecryptor{
		keys:                keys,
		maxEncryptedKeyB64:  DefaultMaxEncryptedKeyB64,
		maxFieldValueBase64: DefaultMaxCiphertextBytes,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// requestContext holds everything derived once per request (the
// unwrapped AES key and the header-declared nonce/timestamp) so
// decrypting several fields doesn't repeat RSA-OAEP or header parsing.
// Obtained via FieldDecryptor.Open.
type requestContext struct {
	aesKey    []byte
	nonce     []byte
	timestamp int64
}

// Open validates headers, resolves/unwraps the per-request AES-256 key,
// and decodes the header nonce/timestamp — everything in
// encryption-steerfile.md §15 steps 1–6. Call Decrypt on the result for
// each field from the SAME request; do not reuse a Session across
// requests (a new one carries a new AES key/nonce/timestamp).
func (d *FieldDecryptor) Open(ctx context.Context, h FieldHeaders) (*FieldSession, error) {
	rc, err := d.openRequestContext(ctx, h)
	if err != nil {
		return nil, err
	}
	return &FieldSession{rc: rc}, nil
}

func (d *FieldDecryptor) openRequestContext(ctx context.Context, h FieldHeaders) (*requestContext, error) {
	switch {
	case h.Version == "":
		return nil, newError(ErrCodeUnsupportedVersion, "missing %s header", HeaderEncryptionVersion)
	case h.KeyID == "":
		return nil, newError(ErrCodeUnknownKeyID, "missing %s header", HeaderEncryptionKid)
	case h.EncryptedKey == "":
		return nil, newError(ErrCodeMalformedEnvelope, "missing %s header", HeaderEncryptionKey)
	case h.Nonce == "":
		return nil, newError(ErrCodeInvalidNonce, "missing %s header", HeaderNonce)
	case h.Timestamp == "":
		return nil, newError(ErrCodeInvalidTimestamp, "missing %s header", HeaderTimestamp)
	}
	if h.Version != FieldEncryptionVersion1 {
		return nil, newError(ErrCodeUnsupportedVersion, "version %q not accepted", h.Version)
	}
	if len(h.EncryptedKey) > d.maxEncryptedKeyB64 {
		return nil, newError(ErrCodePayloadTooLarge, "%s length %d exceeds max %d", HeaderEncryptionKey, len(h.EncryptedKey), d.maxEncryptedKeyB64)
	}

	nonce, err := b64.DecodeString(h.Nonce)
	if err != nil {
		return nil, newError(ErrCodeInvalidNonce, "%s: invalid base64url: %v", HeaderNonce, err)
	}
	if len(nonce) != fieldNonceSize {
		return nil, newError(ErrCodeInvalidNonce, "%s length %d, want %d", HeaderNonce, len(nonce), fieldNonceSize)
	}
	timestamp, err := strconv.ParseInt(h.Timestamp, 10, 64)
	if err != nil {
		return nil, newError(ErrCodeInvalidTimestamp, "%s: not a valid unix timestamp: %v", HeaderTimestamp, err)
	}

	privAny, err := d.keys.GetPrivateKey(ctx, h.KeyID)
	if err != nil {
		if _, ok := CodeOf(err); ok {
			return nil, err
		}
		return nil, newError(ErrCodeKeyProviderUnavailable, "key provider: %v", err)
	}
	priv, ok := privAny.(*rsa.PrivateKey)
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "kid %q is not an RSA key", h.KeyID)
	}

	wrapped, err := b64.DecodeString(h.EncryptedKey)
	if err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "%s: invalid base64url: %v", HeaderEncryptionKey, err)
	}
	// RSA-OAEP with SHA-256 for both the digest and MGF1 hash
	// (RSA-OAEP-256), per §13.
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, wrapped, nil)
	if err != nil {
		return nil, newError(ErrCodeKeyUnwrapFailed, "rsa-oaep unwrap failed")
	}
	if len(aesKey) != aesKeySize {
		return nil, newError(ErrCodeKeyUnwrapFailed, "unwrapped key length %d, want %d", len(aesKey), aesKeySize)
	}

	return &requestContext{aesKey: aesKey, nonce: nonce, timestamp: timestamp}, nil
}

// FieldSession holds the per-request AES key and header nonce/timestamp
// so multiple fields from one request can be decrypted without
// repeating RSA-OAEP unwrap or header parsing. Obtained via
// FieldDecryptor.Open. Not safe to reuse across requests.
type FieldSession struct {
	rc *requestContext
}

// Decrypt authenticates and decrypts a single field's encoded value —
// e.g. dto.Username or dto.Password directly, with no need to marshal
// the whole request body into a map first. Returns "" with no error for
// an empty input string, since an empty/optional field is not itself
// an encryption failure; callers that require the field to be present
// should check that before calling Decrypt.
func (s *FieldSession) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	return decryptFieldValue(s.rc, encoded)
}

// decryptFieldValue implements encryption-steerfile.md §15 steps 8–12
// for one field value: decode, split into
// IV||Ciphertext||Tag||Nonce||Timestamp, verify the embedded
// nonce/timestamp against the ones from the request headers, then
// AES-256-GCM authenticate and decrypt IV||Ciphertext||Tag.
func decryptFieldValue(rc *requestContext, encoded string) (string, error) {
	raw, err := b64.DecodeString(encoded)
	if err != nil {
		return "", newError(ErrCodeInvalidEncryptedValue, "field value: invalid base64url: %v", err)
	}
	if len(raw) > DefaultMaxCiphertextBytes {
		return "", newError(ErrCodePayloadTooLarge, "field value size %d exceeds max %d", len(raw), DefaultMaxCiphertextBytes)
	}
	if len(raw) < fieldFixedOverhead {
		return "", newError(ErrCodeInvalidEncryptedValue, "field value too short: got %d bytes, need at least %d", len(raw), fieldFixedOverhead)
	}

	ciphertextLen := len(raw) - fieldFixedOverhead
	iv := raw[:aesGCMNonceSize]
	ciphertextAndTag := raw[aesGCMNonceSize : aesGCMNonceSize+ciphertextLen+aesGCMTagSize]
	embeddedNonce := raw[aesGCMNonceSize+ciphertextLen+aesGCMTagSize : aesGCMNonceSize+ciphertextLen+aesGCMTagSize+fieldNonceSize]
	embeddedTimestamp := int64(binary.BigEndian.Uint64(raw[len(raw)-fieldTimestampSize:]))

	if !constantTimeEqual(embeddedNonce, rc.nonce) {
		return "", newError(ErrCodeInvalidNonce, "embedded nonce does not match %s header", HeaderNonce)
	}
	if embeddedTimestamp != rc.timestamp {
		return "", newError(ErrCodeInvalidTimestamp, "embedded timestamp does not match %s header", HeaderTimestamp)
	}

	block, err := aes.NewCipher(rc.aesKey)
	if err != nil {
		return "", newError(ErrCodeAuthenticationFailed, "aes cipher init: %v", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, aesGCMNonceSize)
	if err != nil {
		return "", newError(ErrCodeAuthenticationFailed, "gcm init: %v", err)
	}
	plaintext, err := gcm.Open(nil, iv, ciphertextAndTag, nil) // no AAD, per §9/§10
	if err != nil {
		return "", newError(ErrCodeAuthenticationFailed, "gcm authentication failed")
	}
	if len(plaintext) > DefaultMaxPlaintextBytes {
		return "", newError(ErrCodePayloadTooLarge, "plaintext size %d exceeds max %d", len(plaintext), DefaultMaxPlaintextBytes)
	}
	return string(plaintext), nil
}

// DecryptFields decrypts every name in fields that is present in body
// and holds a JSON string, replacing it in place with the decrypted
// plaintext (also as a JSON string). Fields absent from body are
// skipped — callers that need a field to be mandatory should check its
// presence themselves. body is mutated and returned for convenience.
//
// Prefer Open+FieldSession.Decrypt when the caller already has typed
// DTO fields (dto.Username, dto.Password); use DecryptFields/
// DecryptJSONFields when working with a raw JSON body map instead.
func (d *FieldDecryptor) DecryptFields(ctx context.Context, headers FieldHeaders, body map[string]json.RawMessage, fields []string) (map[string]json.RawMessage, error) {
	rc, err := d.openRequestContext(ctx, headers)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		raw, ok := body[field]
		if !ok {
			continue
		}
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return nil, newError(ErrCodeInvalidEncryptedValue, "field %q is not a JSON string", field)
		}
		plaintext, err := decryptFieldValue(rc, encoded)
		if err != nil {
			return nil, err
		}
		replaced, err := json.Marshal(plaintext)
		if err != nil {
			return nil, newError(ErrCodeInvalidEncryptedValue, "field %q: re-encode: %v", field, err)
		}
		body[field] = replaced
	}
	return body, nil
}

// DecryptJSONFields is the convenience entry point for HTTP middleware
// (encryption-steerfile.md §15 step 14, "Pass the restored request body
// to the existing application layer"): unmarshal rawBody, decrypt the
// configured fields in place, and re-marshal, yielding JSON with the
// same shape the frontend started from — ready to json.Unmarshal into
// an unchanged application request struct.
func (d *FieldDecryptor) DecryptJSONFields(ctx context.Context, headers FieldHeaders, rawBody []byte, fields []string) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil, newError(ErrCodeInvalidEncryptedValue, "request body: %v", err)
	}
	if _, err := d.DecryptFields(ctx, headers, body, fields); err != nil {
		return nil, err
	}
	out, err := json.Marshal(body)
	if err != nil {
		return nil, newError(ErrCodeInvalidEncryptedValue, "re-encode body: %v", err)
	}
	return out, nil
}

// constantTimeEqual reports whether a and b are equal, in time
// independent of where they first differ. The embedded nonce is not
// itself secret, but comparing it the same way as any other
// security-relevant equality check is cheap and avoids having to
// re-justify a plain == later if this comparison is ever repurposed
// for something that is secret.
func constantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
