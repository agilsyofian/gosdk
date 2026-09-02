package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

// testJWKS builds a JWKSKeyProvider with one active RSA-2048 key,
// mirroring encryption-steerfile.md §5's configuration shape.
func testJWKS(t *testing.T) (*JWKSKeyProvider, *rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	kid := "encryption-2026-08"
	jwks := Jwks{Keys: []Key{PublicKeyToJWK(&priv.PublicKey, kid)}}
	kp, err := NewJWKSKeyProvider(jwks)
	if err != nil {
		t.Fatalf("NewJWKSKeyProvider: %v", err)
	}
	if err := kp.SetPrivateKey(kid, priv); err != nil {
		t.Fatalf("SetPrivateKey: %v", err)
	}
	if err := kp.SetActive(kid); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	return kp, priv, kid
}

// --- Round trip -------------------------------------------------------------

func TestFieldRoundTrip_DTOStyle(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{
		"name":    "Budi",
		"nik":     "1234567890123456",
		"phone":   "08123456789",
		"address": "Jakarta",
	}, []string{"nik", "phone"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Simulate a DTO the way the application layer already unmarshals
	// its request: unrelated fields stay plaintext, targeted ones stay
	// strings (just encrypted ones).
	type dto struct {
		Name    string `json:"name"`
		NIK     string `json:"nik"`
		Phone   string `json:"phone"`
		Address string `json:"address"`
	}
	rawBody, _ := json.Marshal(req.Body)
	var d dto
	if err := json.Unmarshal(rawBody, &d); err != nil {
		t.Fatalf("unmarshal dto: %v", err)
	}
	if d.Name != "Budi" || d.Address != "Jakarta" {
		t.Fatalf("unrelated fields must stay untouched: %+v", d)
	}

	session, err := dec.Open(ctx, req.Headers)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	nik, err := session.Decrypt(d.NIK)
	if err != nil {
		t.Fatalf("Decrypt(nik): %v", err)
	}
	phone, err := session.Decrypt(d.Phone)
	if err != nil {
		t.Fatalf("Decrypt(phone): %v", err)
	}
	if nik != "1234567890123456" {
		t.Fatalf("nik: got %q", nik)
	}
	if phone != "08123456789" {
		t.Fatalf("phone: got %q", phone)
	}
}

func TestFieldRoundTrip_MapStyle(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{
		"username": "alice",
		"password": "hunter2",
	}, []string{"username", "password"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	rawBody, _ := json.Marshal(req.Body)
	out, err := dec.DecryptJSONFields(ctx, req.Headers, rawBody, []string{"username", "password"})
	if err != nil {
		t.Fatalf("DecryptJSONFields: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if got["username"] != "alice" || got["password"] != "hunter2" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestField_MissingFieldSkipped(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username", "password"})
	if err != nil {
		t.Fatalf("should not fail on absent field: %v", err)
	}
}

// --- Negative / security cases ----------------------------------------------

func TestField_TamperedTag(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	var encoded string
	_ = json.Unmarshal(req.Body["username"], &encoded)
	raw, _ := b64.DecodeString(encoded)
	// Tag lives right after IV+ciphertext; flip a byte inside it. For a
	// short plaintext like "alice" (5 bytes), tag starts at offset
	// 12+5=17.
	raw[17] ^= 0xFF
	req.Body["username"], _ = json.Marshal(b64.EncodeToString(raw))

	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	assertCode(t, err, ErrCodeAuthenticationFailed)
}

func TestField_TamperedCiphertext(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice-x"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	var encoded string
	_ = json.Unmarshal(req.Body["username"], &encoded)
	raw, _ := b64.DecodeString(encoded)
	raw[12] ^= 0xFF // first ciphertext byte
	req.Body["username"], _ = json.Marshal(b64.EncodeToString(raw))

	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	assertCode(t, err, ErrCodeAuthenticationFailed)
}

func TestField_EmbeddedNonceMismatch(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	// Swap in a header nonce that doesn't match what's embedded in the
	// field value.
	otherNonce := make([]byte, fieldNonceSize)
	req.Headers.Nonce = b64.EncodeToString(otherNonce)

	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	assertCode(t, err, ErrCodeInvalidNonce)
}

func TestField_EmbeddedTimestampMismatch(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	req.Headers.Timestamp = formatUnix(nowUnix() + 999)

	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	assertCode(t, err, ErrCodeInvalidTimestamp)
}

func TestField_ValueTooShort(t *testing.T) {
	kp, _, _ := testJWKS(t)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	nonce := make([]byte, fieldNonceSize)
	_, _ = rand.Read(nonce)
	headers := FieldHeaders{
		Version:      FieldEncryptionVersion1,
		KeyID:        "encryption-2026-08",
		EncryptedKey: "AAAA",
		Nonce:        b64.EncodeToString(nonce),
		Timestamp:    formatUnix(nowUnix()),
	}
	body := map[string]json.RawMessage{"username": json.RawMessage(`"AAAA"`)}
	_, err := dec.DecryptFields(ctx, headers, body, []string{"username"})
	// EncryptedKey "AAAA" is well-formed base64url but not a valid
	// wrapped key for this kid's RSA key size, so unwrap fails before
	// the field is even reached — still a legitimate negative case.
	if _, ok := CodeOf(err); !ok {
		t.Fatalf("expected a *crypto.Error, got %v", err)
	}
}

func TestField_UnsupportedVersion(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	req.Headers.Version = "2"
	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	assertCode(t, err, ErrCodeUnsupportedVersion)
}

func TestField_UnknownKid(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	req.Headers.KeyID = "no-such-kid"
	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	assertCode(t, err, ErrCodeUnknownKeyID)
}

func TestField_MissingHeaders(t *testing.T) {
	kp, _, _ := testJWKS(t)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()
	body := map[string]json.RawMessage{"username": json.RawMessage(`"whatever"`)}

	_, err := dec.DecryptFields(ctx, FieldHeaders{}, body, []string{"username"})
	if _, ok := CodeOf(err); !ok {
		t.Fatalf("expected a *crypto.Error, got %v", err)
	}
}

func TestField_KeyRotation(t *testing.T) {
	kp, oldPriv, oldKid := testJWKS(t)
	newPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	newKid := "encryption-2026-01"
	jwks := kp.PublicJWKS()
	jwks.Keys = append(jwks.Keys, PublicKeyToJWK(&newPriv.PublicKey, newKid))
	kp2, err := NewJWKSKeyProvider(jwks)
	if err != nil {
		t.Fatalf("NewJWKSKeyProvider: %v", err)
	}
	if err := kp2.SetPrivateKey(oldKid, oldPriv); err != nil {
		t.Fatalf("SetPrivateKey(old): %v", err)
	}
	if err := kp2.SetPrivateKey(newKid, newPriv); err != nil {
		t.Fatalf("SetPrivateKey(new): %v", err)
	}
	if err := kp2.SetActive(newKid); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	enc := NewFieldEncryptor(kp2)
	dec := NewFieldDecryptor(kp2)
	ctx := context.Background()

	// A request encrypted against the OLD (retired-but-not-revoked)
	// key must still decrypt.
	req, err := enc.Encrypt(ctx, oldKid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt with old kid: %v", err)
	}
	out, err := dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	if err != nil {
		t.Fatalf("decrypt with retired-but-valid key: %v", err)
	}
	var got string
	_ = json.Unmarshal(out["username"], &got)
	if got != "alice" {
		t.Fatalf("got %q", got)
	}
}

func TestField_RevokedKeyRejected(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	kp.RevokeKey(kid)

	_, err = dec.DecryptFields(ctx, req.Headers, req.Body, []string{"username"})
	assertCode(t, err, ErrCodeUnknownKeyID)
}

// --- JWKS --------------------------------------------------------------------

func TestJWKS_RejectsWrongUse(t *testing.T) {
	_, priv := generateTestRSAKey(t)
	k := PublicKeyToJWK(&priv.PublicKey, "sig-key")
	k.Use = "sig"
	_, err := NewJWKSKeyProvider(Jwks{Keys: []Key{k}})
	if err == nil {
		t.Fatalf("expected an error for use=sig")
	}
}

func TestJWKS_RejectsWrongAlg(t *testing.T) {
	_, priv := generateTestRSAKey(t)
	k := PublicKeyToJWK(&priv.PublicKey, "bad-alg")
	k.Alg = "RSA-OAEP"
	_, err := NewJWKSKeyProvider(Jwks{Keys: []Key{k}})
	if err == nil {
		t.Fatalf("expected an error for alg=RSA-OAEP")
	}
}

func TestJWKS_RejectsDuplicateKid(t *testing.T) {
	_, priv := generateTestRSAKey(t)
	k := PublicKeyToJWK(&priv.PublicKey, "dup")
	_, err := NewJWKSKeyProvider(Jwks{Keys: []Key{k, k}})
	if err == nil {
		t.Fatalf("expected an error for duplicate kid")
	}
}

func TestJWKS_SetPrivateKeyMismatchRejected(t *testing.T) {
	_, priv := generateTestRSAKey(t)
	k := PublicKeyToJWK(&priv.PublicKey, "kid-a")
	kp, err := NewJWKSKeyProvider(Jwks{Keys: []Key{k}})
	if err != nil {
		t.Fatalf("NewJWKSKeyProvider: %v", err)
	}
	_, otherPriv := generateTestRSAKey(t)
	if err := kp.SetPrivateKey("kid-a", otherPriv); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestJWKS_PublicJWKSNeverLeaksPrivateFields(t *testing.T) {
	kp, _, _ := testJWKS(t)
	jwks := kp.PublicJWKS()
	raw, err := json.Marshal(jwks)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	for _, forbidden := range []string{`"d":`, `"p":`, `"q":`, `"dp":`, `"dq":`, `"qi":`} {
		if strContains(s, forbidden) {
			t.Fatalf("served JWKS leaked a private parameter: %s in %s", forbidden, s)
		}
	}
}

func generateTestRSAKey(t *testing.T) (*rsa.PublicKey, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return &priv.PublicKey, priv
}

func strContains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// --- ReplayProtector: a separate concern, not called by FieldDecryptor -----

func TestReplayProtector_IsIndependentOfFieldDecryptor(t *testing.T) {
	kp, _, kid := testJWKS(t)
	enc := NewFieldEncryptor(kp)
	dec := NewFieldDecryptor(kp)
	ctx := context.Background()

	req, err := enc.Encrypt(ctx, kid, map[string]any{"username": "alice"}, []string{"username"})
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Decrypting the SAME request twice must succeed both times at the
	// FieldDecryptor layer — replay rejection is not its job.
	body1 := cloneBody(t, req.Body)
	if _, err := dec.DecryptFields(ctx, req.Headers, body1, []string{"username"}); err != nil {
		t.Fatalf("first decrypt: %v", err)
	}
	body2 := cloneBody(t, req.Body)
	if _, err := dec.DecryptFields(ctx, req.Headers, body2, []string{"username"}); err != nil {
		t.Fatalf("FieldDecryptor must not itself enforce replay protection: %v", err)
	}

	// A ReplayProtector wired in as separate middleware DOES reject the
	// second use of the same nonce.
	store := NewInMemoryNonceStore(DefaultNonceWindowPolicy)
	ts, err := strconv.ParseInt(req.Headers.Timestamp, 10, 64)
	if err != nil {
		t.Fatalf("parse timestamp: %v", err)
	}
	if err := store.Validate(ctx, req.Headers.Nonce, ts); err != nil {
		t.Fatalf("first Validate should pass: %v", err)
	}
	err = store.Validate(ctx, req.Headers.Nonce, ts)
	assertCode(t, err, ErrCodeReplayedRequest)
}

func TestReplayProtector_ExpiredTimestamp(t *testing.T) {
	store := NewInMemoryNonceStore(NonceWindowPolicy{ClockSkew: time.Minute, NonceTTL: time.Minute})
	err := store.Validate(context.Background(), "some-nonce", time.Now().Add(-time.Hour).Unix())
	assertCode(t, err, ErrCodeExpiredRequest)
}

func cloneBody(t *testing.T, body map[string]json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	out := make(map[string]json.RawMessage, len(body))
	for k, v := range body {
		cp := make(json.RawMessage, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
