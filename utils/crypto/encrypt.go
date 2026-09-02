package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Encryptor builds envelopes by generating a fresh AES-256-GCM key,
// sealing the plaintext, and wrapping the AES key with the recipient's
// RSA-OAEP public key, per STEERFILE.md §5/§12.
//
// This is the same operation the frontend performs with the Web Crypto
// API (see frontend/crypto.ts). The Go implementation exists so:
//   - a Go backend can encrypt outbound envelopes to another service
//     that also speaks this protocol, and
//   - interoperability/security test suites can generate deterministic
//     test vectors without depending on a JS runtime (STEERFILE.md §23/§24).
//
// It MUST NOT be used to embed a long-lived symmetric key in a browser
// bundle — that responsibility, and the "frontend is not a secure
// secret store" constraint, stays exactly as described in
// STEERFILE.md §6.2.
type Encryptor struct {
	keys KeyProvider
}

// NewEncryptor builds an Encryptor that resolves recipient public keys
// through keys.
func NewEncryptor(keys KeyProvider) *Encryptor {
	return &Encryptor{keys: keys}
}

// Encrypt builds a versioned Envelope for plaintext, addressed to
// meta.KeyID (or the provider's current active key if meta.KeyID is
// empty). meta.RequestID and a fresh timestamp are populated
// automatically if RequestID is empty; callers that need a
// caller-chosen request_id (e.g. to correlate with an outer request)
// should set it explicitly before calling.
//
// A fresh AES-256 key and 96-bit nonce are generated for every call —
// nonces and keys are never reused across invocations, per STEERFILE.md §5.
func (e *Encryptor) Encrypt(ctx context.Context, plaintext []byte, meta Metadata) (*Envelope, error) {
	if len(plaintext) > DefaultMaxPlaintextBytes {
		return nil, newError(ErrCodePayloadTooLarge, "plaintext size %d exceeds max %d", len(plaintext), DefaultMaxPlaintextBytes)
	}

	keyID := meta.KeyID
	if keyID == "" {
		active, err := e.keys.ActiveKeyID(ctx)
		if err != nil {
			return nil, err
		}
		keyID = active
	}
	pubAny, err := e.keys.GetPublicKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "key_id %q is not an RSA public key", keyID)
	}

	if meta.RequestID == "" {
		id, err := GenerateRequestID()
		if err != nil {
			return nil, newError(ErrCodeKeyProviderUnavailable, "generate request_id: %v", err)
		}
		meta.RequestID = id
	}
	meta.Version = ProtocolVersion1
	meta.KeyID = keyID

	aesKey := make([]byte, aesKeySize)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, newError(ErrCodeKeyProviderUnavailable, "generate aes key: %v", err)
	}
	iv := make([]byte, aesGCMNonceSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, newError(ErrCodeKeyProviderUnavailable, "generate iv: %v", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, newError(ErrCodeAuthenticationFailed, "aes cipher init: %v", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, aesGCMNonceSize)
	if err != nil {
		return nil, newError(ErrCodeAuthenticationFailed, "gcm init: %v", err)
	}

	aad := buildAAD(meta)
	sealed := gcm.Seal(nil, iv, plaintext, aad) // ciphertext || tag
	ciphertext := sealed[:len(sealed)-aesGCMTagSize]
	tag := sealed[len(sealed)-aesGCMTagSize:]

	wrappedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return nil, newError(ErrCodeKeyUnwrapFailed, "rsa-oaep wrap: %v", err)
	}

	env := &Envelope{
		Version:      ProtocolVersion1,
		Algorithm:    AlgorithmA256GCM,
		KeyID:        keyID,
		EncryptedKey: b64.EncodeToString(wrappedKey),
		IV:           b64.EncodeToString(iv),
		Ciphertext:   b64.EncodeToString(ciphertext),
		Tag:          b64.EncodeToString(tag),
		Timestamp:    time.Now().Unix(),
		RequestID:    meta.RequestID,
	}
	return env, nil
}

// GenerateRequestID returns a fresh, unpredictable request identifier
// suitable for the envelope's request_id field and for replay tracking.
// Format: 8 hex timestamp seconds + 16 hex random bytes, e.g.
// "68b1a2c3-9f2e1a7b4c3d5e6f". This is not a spec-mandated format (the
// protocol treats request_id as an opaque string); callers may
// substitute a ULID or UUIDv4 generator if their stack already has one.
func GenerateRequestID() (string, error) {
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x-%s", time.Now().Unix(), hex.EncodeToString(rnd[:])), nil
}
