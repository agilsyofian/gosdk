package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"time"
)

// Session holds key material scoped to a single request, produced by
// Decryptor.DecryptForResponse. ResponseKey is NOT the raw request AES
// key: it is derived via HKDF-SHA256 so a response never reuses the
// exact key bytes used for the request, per STEERFILE.md §13 ("do NOT
// simply reuse the same symmetric key indefinitely"). Session is only
// meaningful for the lifetime of the request/response cycle and MUST
// NOT be persisted or reused across requests.
type Session struct {
	ResponseKey [aesKeySize]byte
}

// deriveResponseKey derives a response-only key from the request's AES
// key using HKDF-SHA256, salted with the request_id so every request
// yields an independent response key even if (hypothetically) the same
// AES key were ever reused.
func deriveResponseKey(requestAESKey []byte, requestID string) [aesKeySize]byte {
	var out [aesKeySize]byte
	r := hkdfSHA256(requestAESKey, []byte(requestID), []byte("fe-be-crypto:response-key:v1"), aesKeySize)
	copy(out[:], r)
	return out
}

// hkdfSHA256 is a minimal HKDF (RFC 5869) implementation using SHA-256,
// avoiding an extra dependency for a single derive-key call. extract
// uses HMAC-SHA256(salt, ikm); expand produces length bytes of output
// keying material bound to info.
func hkdfSHA256(ikm, salt, info []byte, length int) []byte {
	// Extract
	extractor := hmac.New(sha256.New, salt)
	extractor.Write(ikm)
	prk := extractor.Sum(nil)

	// Expand
	var (
		out []byte
		t   []byte
		ctr byte = 1
	)
	for len(out) < length {
		mac := hmac.New(sha256.New, prk)
		mac.Write(t)
		mac.Write(info)
		mac.Write([]byte{ctr})
		t = mac.Sum(nil)
		out = append(out, t...)
		ctr++
	}
	return out[:length]
}

// ResponseEnvelope is the wire format for a backend->frontend encrypted
// response. It omits encrypted_key: the AES key was already established
// for this request/response cycle (as a derived Session.ResponseKey), so
// there is nothing to RSA-wrap. Frontend derives the identical
// ResponseKey from the same request AES key and request_id.
type ResponseEnvelope struct {
	Version    string `json:"version"`
	Algorithm  string `json:"algorithm"`
	KeyID      string `json:"key_id"`
	IV         string `json:"iv"`
	Ciphertext string `json:"ciphertext"`
	Tag        string `json:"tag"`
	Timestamp  int64  `json:"timestamp"`
	RequestID  string `json:"request_id"`
}

// EncryptResponse seals plaintext under session.ResponseKey, binding it
// to meta via AAD (method/path SHOULD reflect the original request per
// STEERFILE.md §13's "bind the response to request context"). Use only
// when a concrete requirement calls for backend->frontend
// application-layer encryption; for ordinary APIs TLS is sufficient.
func EncryptResponse(session *Session, plaintext []byte, meta Metadata) (*ResponseEnvelope, error) {
	if len(plaintext) > DefaultMaxPlaintextBytes {
		return nil, newError(ErrCodePayloadTooLarge, "plaintext size %d exceeds max %d", len(plaintext), DefaultMaxPlaintextBytes)
	}
	iv := make([]byte, aesGCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, newError(ErrCodeKeyProviderUnavailable, "generate iv: %v", err)
	}
	block, err := aes.NewCipher(session.ResponseKey[:])
	if err != nil {
		return nil, newError(ErrCodeAuthenticationFailed, "aes cipher init: %v", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, aesGCMNonceSize)
	if err != nil {
		return nil, newError(ErrCodeAuthenticationFailed, "gcm init: %v", err)
	}
	meta.Version = ProtocolVersion1
	aad := buildAAD(meta)
	sealed := gcm.Seal(nil, iv, plaintext, aad)
	ciphertext := sealed[:len(sealed)-aesGCMTagSize]
	tag := sealed[len(sealed)-aesGCMTagSize:]

	return &ResponseEnvelope{
		Version:    ProtocolVersion1,
		Algorithm:  AlgorithmA256GCM,
		KeyID:      meta.KeyID,
		IV:         b64.EncodeToString(iv),
		Ciphertext: b64.EncodeToString(ciphertext),
		Tag:        b64.EncodeToString(tag),
		Timestamp:  time.Now().Unix(),
		RequestID:  meta.RequestID,
	}, nil
}

// DecryptResponse is the frontend-equivalent counterpart implemented in
// Go purely for interoperability test vectors (STEERFILE.md §23/§24);
// a real frontend performs this with the Web Crypto API, see
// frontend/crypto.ts.
func DecryptResponse(session *Session, env *ResponseEnvelope, meta Metadata) ([]byte, error) {
	if env.Algorithm != AlgorithmA256GCM {
		return nil, newError(ErrCodeUnsupportedAlgorithm, "algorithm %q not accepted", env.Algorithm)
	}
	iv, err := b64.DecodeString(env.IV)
	if err != nil || len(iv) != aesGCMNonceSize {
		return nil, newError(ErrCodeMalformedEnvelope, "invalid iv")
	}
	ct, err := b64.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "invalid ciphertext")
	}
	tag, err := b64.DecodeString(env.Tag)
	if err != nil || len(tag) != aesGCMTagSize {
		return nil, newError(ErrCodeMalformedEnvelope, "invalid tag")
	}
	block, err := aes.NewCipher(session.ResponseKey[:])
	if err != nil {
		return nil, newError(ErrCodeAuthenticationFailed, "aes cipher init: %v", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, aesGCMNonceSize)
	if err != nil {
		return nil, newError(ErrCodeAuthenticationFailed, "gcm init: %v", err)
	}
	meta.Version = env.Version
	meta.KeyID = env.KeyID
	meta.RequestID = env.RequestID
	aad := buildAAD(meta)
	plaintext, err := gcm.Open(nil, iv, append(ct, tag...), aad)
	if err != nil {
		return nil, newError(ErrCodeAuthenticationFailed, "gcm authentication failed")
	}
	return plaintext, nil
}
