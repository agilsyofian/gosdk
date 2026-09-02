package crypto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

// ProtocolVersion identifies the wire protocol described in
// STEERFILE.md. New protocol versions (breaking envelope/AAD/algorithm
// changes) MUST get a new constant rather than silently changing the
// behavior of an existing one — see STEERFILE.md §25.
const ProtocolVersion1 = "1"

// AlgorithmA256GCM is the only algorithm identifier this backend accepts
// for the symmetric payload cipher, per STEERFILE.md §5.
const AlgorithmA256GCM = "A256GCM"

// Size limits enforced before any attacker-controlled length is used to
// allocate memory or drive decryption, per STEERFILE.md §20. These are
// generous defaults for typical API payloads; override via
// DecryptorOption if a specific deployment needs different bounds.
const (
	DefaultMaxEnvelopeBytes   = 1 << 20 // 1 MiB serialized envelope
	DefaultMaxEncryptedKeyB64 = 2048    // RSA-4096-OAEP wrapped key, base64url
	DefaultMaxCiphertextBytes = 512 * 1024
	DefaultMaxPlaintextBytes  = 512 * 1024
	aesGCMNonceSize           = 12 // 96-bit nonce, per STEERFILE.md §5
	aesGCMTagSize             = 16 // 128-bit tag, per STEERFILE.md §5
	aesKeySize                = 32 // AES-256
)

// Envelope is the versioned wire format exchanged between frontend and
// backend, matching STEERFILE.md §7. All binary fields are Base64URL
// without padding (STEERFILE.md §8). Tag is optional on the wire: some
// implementations append the GCM tag to Ciphertext instead of
// representing it separately, so a zero-length Tag is valid and the
// decryptor treats the trailing bytes of Ciphertext as the tag in that
// case (see decodeCiphertextAndTag).
type Envelope struct {
	Version      string `json:"version"`
	Algorithm    string `json:"algorithm"`
	KeyID        string `json:"key_id"`
	EncryptedKey string `json:"encrypted_key"`
	IV           string `json:"iv"`
	Ciphertext   string `json:"ciphertext"`
	Tag          string `json:"tag,omitempty"`
	Timestamp    int64  `json:"timestamp"`
	RequestID    string `json:"request_id"`
}

var b64 = base64.URLEncoding.WithPadding(base64.NoPadding)

// ParseEnvelope decodes and structurally validates raw JSON bytes into an
// Envelope. It enforces size limits before touching any cryptographic
// material, per STEERFILE.md §20. It does NOT validate the protocol
// version, algorithm, or key_id against policy — that is Decryptor's
// job, since only Decryptor knows the currently accepted set.
func ParseEnvelope(raw []byte, maxEnvelopeBytes int) (*Envelope, error) {
	if maxEnvelopeBytes <= 0 {
		maxEnvelopeBytes = DefaultMaxEnvelopeBytes
	}
	if len(raw) == 0 {
		return nil, newError(ErrCodeMalformedEnvelope, "empty body")
	}
	if len(raw) > maxEnvelopeBytes {
		return nil, newError(ErrCodePayloadTooLarge, "envelope size %d exceeds max %d", len(raw), maxEnvelopeBytes)
	}

	var env Envelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "json decode: %v", err)
	}

	if err := env.validateStructure(); err != nil {
		return nil, err
	}
	return &env, nil
}

// validateStructure checks that every required field is present and
// every binary field is valid Base64URL, without yet interpreting the
// bytes cryptographically or checking them against policy (version,
// algorithm, key_id).
func (e *Envelope) validateStructure() error {
	switch {
	case e.Version == "":
		return newError(ErrCodeMalformedEnvelope, "missing version")
	case e.Algorithm == "":
		return newError(ErrCodeMalformedEnvelope, "missing algorithm")
	case e.KeyID == "":
		return newError(ErrCodeMalformedEnvelope, "missing key_id")
	case e.EncryptedKey == "":
		return newError(ErrCodeMalformedEnvelope, "missing encrypted_key")
	case e.IV == "":
		return newError(ErrCodeMalformedEnvelope, "missing iv")
	case e.Ciphertext == "":
		return newError(ErrCodeMalformedEnvelope, "missing ciphertext")
	case e.Timestamp == 0:
		return newError(ErrCodeMalformedEnvelope, "missing timestamp")
	case e.RequestID == "":
		return newError(ErrCodeMalformedEnvelope, "missing request_id")
	}

	if len(e.EncryptedKey) > DefaultMaxEncryptedKeyB64 {
		return newError(ErrCodePayloadTooLarge, "encrypted_key size %d exceeds max %d", len(e.EncryptedKey), DefaultMaxEncryptedKeyB64)
	}
	if _, err := b64.DecodeString(e.EncryptedKey); err != nil {
		return newError(ErrCodeMalformedEnvelope, "encrypted_key: invalid base64url: %v", err)
	}
	iv, err := b64.DecodeString(e.IV)
	if err != nil {
		return newError(ErrCodeMalformedEnvelope, "iv: invalid base64url: %v", err)
	}
	if len(iv) != aesGCMNonceSize {
		return newError(ErrCodeMalformedEnvelope, "iv length %d, want %d", len(iv), aesGCMNonceSize)
	}
	ct, err := b64.DecodeString(e.Ciphertext)
	if err != nil {
		return newError(ErrCodeMalformedEnvelope, "ciphertext: invalid base64url: %v", err)
	}
	if len(ct) > DefaultMaxCiphertextBytes {
		return newError(ErrCodePayloadTooLarge, "ciphertext size %d exceeds max %d", len(ct), DefaultMaxCiphertextBytes)
	}
	if e.Tag != "" {
		tag, err := b64.DecodeString(e.Tag)
		if err != nil {
			return newError(ErrCodeMalformedEnvelope, "tag: invalid base64url: %v", err)
		}
		if len(tag) != aesGCMTagSize {
			return newError(ErrCodeMalformedEnvelope, "tag length %d, want %d", len(tag), aesGCMTagSize)
		}
	} else if len(ct) < aesGCMTagSize {
		// Tag not represented separately: it must be the trailing
		// aesGCMTagSize bytes of ciphertext (Go's crypto/cipher AEAD
		// convention). Reject anything too short to contain one.
		return newError(ErrCodeMalformedEnvelope, "ciphertext too short to contain trailing tag")
	}
	return nil
}

// MarshalJSON-equivalent helper used by the encryptor to produce wire
// bytes. Kept as a plain method (not json.Marshaler) so field order in
// Envelope's struct tags stays the single source of truth.
func (e *Envelope) Bytes() ([]byte, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "marshal envelope: %v", err)
	}
	return b, nil
}
