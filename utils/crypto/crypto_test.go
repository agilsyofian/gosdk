package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"
	"time"
)

func testKeyProvider(t *testing.T) (*StaticKeyProvider, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	kp := NewStaticKeyProvider()
	kp.AddKey("test-key-2026-01", priv, true)
	return kp, priv
}

func testMeta() Metadata {
	return Metadata{
		Method:      "POST",
		Path:        "/api/v1/example",
		ContentType: "application/json",
	}
}

func newTestDecryptor(kp KeyProvider) *Decryptor {
	return NewDecryptor(kp, WithReplayStore(NewInMemoryReplayStore(), DefaultReplayPolicy))
}

// --- Unit tests: valid path -------------------------------------------------

func TestEncryptDecryptRoundTrip(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	plaintext := []byte(`{"account":"1234","amount":100000}`)
	env, err := enc.Encrypt(ctx, plaintext, testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := dec.DecryptEnvelope(ctx, env, testMeta())
	if err != nil {
		t.Fatalf("DecryptEnvelope: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("round-trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestEncryptDecrypt_WireBytesRoundTrip(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("hello"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	raw, err := env.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	got, err := dec.Decrypt(ctx, raw, testMeta())
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
}

// --- Unit tests: invalid inputs ---------------------------------------------

func TestDecrypt_InvalidKey(t *testing.T) {
	kp, _ := testKeyProvider(t)
	otherKP, _ := testKeyProvider(t) // different RSA key, same key_id space
	enc := NewEncryptor(otherKP)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	// Force the envelope to claim kp's key_id even though it was wrapped
	// with otherKP's public key, simulating a wrapped-key/key_id mismatch.
	env.KeyID = "test-key-2026-01"

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeKeyUnwrapFailed)
}

func TestDecrypt_InvalidIV_WrongLength(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	env.IV = b64.EncodeToString([]byte("short"))

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeMalformedEnvelope)
}

func TestDecrypt_InvalidAuthenticationTag(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	tag, _ := b64.DecodeString(env.Tag)
	tag[0] ^= 0xFF
	env.Tag = b64.EncodeToString(tag)

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeAuthenticationFailed)
}

func TestDecrypt_ModifiedCiphertext(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret-payload"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	ct, _ := b64.DecodeString(env.Ciphertext)
	ct[0] ^= 0xFF
	env.Ciphertext = b64.EncodeToString(ct)

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeAuthenticationFailed)
}

func TestDecrypt_ModifiedAAD(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	tampered := testMeta()
	tampered.Path = "/api/v1/DIFFERENT-PATH"
	_, err = dec.DecryptEnvelope(ctx, env, tampered)
	assertCode(t, err, ErrCodeAuthenticationFailed)
}

func TestDecrypt_UnsupportedAlgorithm(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	env.Algorithm = "AES-256-CBC"

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeUnsupportedAlgorithm)
}

func TestDecrypt_UnsupportedProtocolVersion(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	env.Version = "99"

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeUnsupportedVersion)
}

func TestDecrypt_UnknownKeyID(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	env.KeyID = "does-not-exist"

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeUnknownKeyID)
}

func TestParseEnvelope_MalformedJSON(t *testing.T) {
	_, err := ParseEnvelope([]byte(`{"version":`), 0)
	assertCode(t, err, ErrCodeMalformedEnvelope)
}

func TestParseEnvelope_MissingFields(t *testing.T) {
	_, err := ParseEnvelope([]byte(`{"version":"1"}`), 0)
	assertCode(t, err, ErrCodeMalformedEnvelope)
}

func TestParseEnvelope_OversizedPayload(t *testing.T) {
	big := make([]byte, 100)
	for i := range big {
		big[i] = 'a'
	}
	body, _ := json.Marshal(map[string]any{
		"version": "1", "algorithm": "A256GCM", "key_id": "k",
		"encrypted_key": "AAAA", "iv": b64.EncodeToString(make([]byte, 12)),
		"ciphertext": b64.EncodeToString(make([]byte, 32)),
		"timestamp":  1, "request_id": "r1",
	})
	_, err := ParseEnvelope(body, len(body)-1) // force size limit below actual
	assertCode(t, err, ErrCodePayloadTooLarge)
}

// --- Security tests: freshness & replay -------------------------------------

func TestDecrypt_ExpiredTimestamp(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	env.Timestamp = time.Now().Add(-1 * time.Hour).Unix()

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeStaleTimestamp)
}

func TestDecrypt_FutureTimestamp(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	env.Timestamp = time.Now().Add(1 * time.Hour).Unix()

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeStaleTimestamp)
}

func TestDecrypt_ReplayDetected(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := dec.DecryptEnvelope(ctx, env, testMeta()); err != nil {
		t.Fatalf("first decrypt should succeed: %v", err)
	}
	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeReplayDetected)
}

func TestDecrypt_ReplayProtectionRequiredByDefault(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := NewDecryptor(kp) // no WithReplayStore, no WithoutReplayProtection
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeReplayDetected)
}

func TestDecrypt_KeyRotation(t *testing.T) {
	kp, _ := testKeyProvider(t)
	oldPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	kp.AddKey("test-key-2025-07", oldPriv, false) // retired, still accepted
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	// Encrypt against the retired key directly.
	retiredKP := NewStaticKeyProvider()
	retiredKP.AddKey("test-key-2025-07", oldPriv, true)
	enc := NewEncryptor(retiredKP)

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := dec.DecryptEnvelope(ctx, env, testMeta())
	if err != nil {
		t.Fatalf("decrypting with retired-but-not-revoked key should succeed: %v", err)
	}
	if string(got) != "secret" {
		t.Fatalf("got %q", got)
	}
}

func TestDecrypt_RevokedKeyRejected(t *testing.T) {
	kp, priv := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	env, err := enc.Encrypt(ctx, []byte("secret"), testMeta())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	kp.RevokeKey("test-key-2026-01")
	_ = priv

	_, err = dec.DecryptEnvelope(ctx, env, testMeta())
	assertCode(t, err, ErrCodeUnknownKeyID)
}

// --- Nonce uniqueness --------------------------------------------------------

func TestEncrypt_NonceUniqueness(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	ctx := context.Background()

	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		env, err := enc.Encrypt(ctx, []byte("payload"), testMeta())
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if seen[env.IV] {
			t.Fatalf("nonce reused: %s", env.IV)
		}
		seen[env.IV] = true
		if seen[env.RequestID] {
			t.Fatalf("request_id reused: %s", env.RequestID)
		}
		seen[env.RequestID] = true
	}
}

// --- Response encryption -----------------------------------------------------

func TestResponseEncryptionRoundTrip(t *testing.T) {
	kp, _ := testKeyProvider(t)
	enc := NewEncryptor(kp)
	dec := newTestDecryptor(kp)
	ctx := context.Background()

	reqMeta := testMeta()
	env, err := enc.Encrypt(ctx, []byte("request-payload"), reqMeta)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	_, session, err := dec.DecryptForResponse(ctx, env, reqMeta)
	if err != nil {
		t.Fatalf("DecryptForResponse: %v", err)
	}

	respMeta := Metadata{Method: "POST", Path: "/api/v1/example", ContentType: "application/json", KeyID: env.KeyID, RequestID: env.RequestID}
	respEnv, err := EncryptResponse(session, []byte("response-payload"), respMeta)
	if err != nil {
		t.Fatalf("EncryptResponse: %v", err)
	}

	got, err := DecryptResponse(session, respEnv, respMeta)
	if err != nil {
		t.Fatalf("DecryptResponse: %v", err)
	}
	if string(got) != "response-payload" {
		t.Fatalf("got %q", got)
	}
}

// --- helpers -----------------------------------------------------------------

func assertCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	got, ok := CodeOf(err)
	if !ok {
		t.Fatalf("expected *crypto.Error with code %s, got %T: %v", want, err, err)
	}
	if got != want {
		t.Fatalf("expected code %s, got %s (%v)", want, got, err)
	}
	// External error message must never leak internal detail.
	if err.Error() != "invalid encrypted payload" {
		t.Fatalf("external error message leaked detail: %q", err.Error())
	}
}
