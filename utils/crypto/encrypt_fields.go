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
	"time"
)

// FieldEncryptor builds per-field encrypted values following
// encryption-steerfile.md §14 exactly. It exists so:
//   - Go-based test suites can generate deterministic interoperability
//     vectors without a JS/Postman runtime (§19's contract, exercised
//     in Go instead), and
//   - a Go service that itself needs to call another backend speaking
//     this protocol can do so.
//
// Production browser clients MUST use the Web Crypto API or an
// established, audited library instead of a bespoke implementation
// (§20) — see frontend/crypto.ts.
type FieldEncryptor struct {
	keys KeyProvider
}

// NewFieldEncryptor builds a FieldEncryptor that resolves the
// recipient's public key through keys.
func NewFieldEncryptor(keys KeyProvider) *FieldEncryptor {
	return &FieldEncryptor{keys: keys}
}

// EncryptedRequest is the result of Encrypt: the (possibly-mutated)
// JSON body plus the headers that must accompany it, per §12.
type EncryptedRequest struct {
	Body    map[string]json.RawMessage
	Headers FieldHeaders
}

// Encrypt performs encryption-steerfile.md §14 steps 1–9 against body:
// resolve the key (kid, or the provider's active key if kid is empty),
// generate one AES-256 key + one nonce + one timestamp for the whole
// request, encrypt each of fields with its own fresh IV, and wrap the
// AES key once with RSA-OAEP-SHA256. body is copied field-by-field into
// the result; the caller's map is not mutated.
func (e *FieldEncryptor) Encrypt(ctx context.Context, kid string, body map[string]any, fields []string) (*EncryptedRequest, error) {
	if kid == "" {
		active, err := e.keys.ActiveKeyID(ctx)
		if err != nil {
			return nil, err
		}
		kid = active
	}
	pubAny, err := e.keys.GetPublicKey(ctx, kid)
	if err != nil {
		return nil, err
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "kid %q is not an RSA public key", kid)
	}

	aesKey := make([]byte, aesKeySize)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, newError(ErrCodeKeyProviderUnavailable, "generate aes key: %v", err)
	}
	nonce := make([]byte, fieldNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, newError(ErrCodeKeyProviderUnavailable, "generate nonce: %v", err)
	}
	timestamp := nowUnix()

	out := make(map[string]json.RawMessage, len(body))
	for k, v := range body {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, newError(ErrCodeInvalidEncryptedValue, "marshal field %q: %v", k, err)
		}
		out[k] = raw
	}

	for _, field := range fields {
		raw, ok := out[field]
		if !ok {
			continue // matches the Postman script's own missing-field behavior
		}
		var plaintext string
		if err := json.Unmarshal(raw, &plaintext); err != nil {
			return nil, newError(ErrCodeInvalidEncryptedValue, "field %q is not a JSON string", field)
		}
		encoded, err := encryptFieldValue(aesKey, nonce, timestamp, plaintext)
		if err != nil {
			return nil, err
		}
		replaced, err := json.Marshal(encoded)
		if err != nil {
			return nil, newError(ErrCodeInvalidEncryptedValue, "field %q: re-encode: %v", field, err)
		}
		out[field] = replaced
	}

	wrappedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, nil)
	if err != nil {
		return nil, newError(ErrCodeKeyUnwrapFailed, "rsa-oaep wrap: %v", err)
	}

	return &EncryptedRequest{
		Body: out,
		Headers: FieldHeaders{
			Version:      FieldEncryptionVersion1,
			KeyID:        kid,
			EncryptedKey: b64.EncodeToString(wrappedKey),
			Nonce:        b64.EncodeToString(nonce),
			Timestamp:    formatUnix(timestamp),
		},
	}, nil
}

// encryptFieldValue builds Base64URL(IV || Ciphertext || Tag || Nonce ||
// Timestamp) for one field, per §10/§14.7.
func encryptFieldValue(aesKey, nonce []byte, timestamp int64, plaintext string) (string, error) {
	iv := make([]byte, aesGCMNonceSize)
	if _, err := rand.Read(iv); err != nil {
		return "", newError(ErrCodeKeyProviderUnavailable, "generate iv: %v", err)
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", newError(ErrCodeAuthenticationFailed, "aes cipher init: %v", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, aesGCMNonceSize)
	if err != nil {
		return "", newError(ErrCodeAuthenticationFailed, "gcm init: %v", err)
	}
	sealed := gcm.Seal(nil, iv, []byte(plaintext), nil) // ciphertext || tag, no AAD

	var ts [fieldTimestampSize]byte
	binary.BigEndian.PutUint64(ts[:], uint64(timestamp))

	combined := make([]byte, 0, len(iv)+len(sealed)+len(nonce)+fieldTimestampSize)
	combined = append(combined, iv...)
	combined = append(combined, sealed...)
	combined = append(combined, nonce...)
	combined = append(combined, ts[:]...)

	return b64.EncodeToString(combined), nil
}

func nowUnix() int64 { return time.Now().Unix() }

func formatUnix(ts int64) string { return strconv.FormatInt(ts, 10) }
