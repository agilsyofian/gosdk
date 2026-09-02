package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
)

// mockFetcher is a test-double PublicKeyFetcher.
type mockFetcher struct {
	keys map[string]*rsa.PublicKey
	err  error
}

func (m *mockFetcher) FetchPublicKey(_ context.Context, kid string) (*rsa.PublicKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	k, ok := m.keys[kid]
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "kid %q not found", kid)
	}
	return k, nil
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return priv
}

// --- Tests ---

func TestRemoteJWKSKeyProvider_GetPublicKey(t *testing.T) {
	priv := generateRSAKey(t)
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{"kid-1": &priv.PublicKey}}

	kp := NewRemoteJWKSKeyProvider(fetcher)

	pub, err := kp.GetPublicKey(context.Background(), "kid-1")
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}
	got, ok := pub.(*rsa.PublicKey)
	if !ok {
		t.Fatal("expected *rsa.PublicKey")
	}
	if got.N.Cmp(priv.PublicKey.N) != 0 || got.E != priv.PublicKey.E {
		t.Error("returned public key does not match injected key")
	}
}

func TestRemoteJWKSKeyProvider_GetPublicKey_UnknownKid(t *testing.T) {
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{}}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	_, err := kp.GetPublicKey(context.Background(), "no-such-kid")
	if err == nil {
		t.Fatal("expected error for unknown kid")
	}
}

func TestRemoteJWKSKeyProvider_GetPublicKey_FetcherError(t *testing.T) {
	fetcher := &mockFetcher{err: errors.New("network error")}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	_, err := kp.GetPublicKey(context.Background(), "any-kid")
	if err == nil {
		t.Fatal("expected error when fetcher fails")
	}
}

func TestRemoteJWKSKeyProvider_GetPrivateKey(t *testing.T) {
	priv := generateRSAKey(t)
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{"kid-1": &priv.PublicKey}}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	if err := kp.SetPrivateKey("kid-1", priv); err != nil {
		t.Fatalf("SetPrivateKey: %v", err)
	}

	got, err := kp.GetPrivateKey(context.Background(), "kid-1")
	if err != nil {
		t.Fatalf("GetPrivateKey: %v", err)
	}
	if got != priv {
		t.Error("returned private key does not match registered key")
	}
}

func TestRemoteJWKSKeyProvider_GetPrivateKey_NotRegistered(t *testing.T) {
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{}}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	_, err := kp.GetPrivateKey(context.Background(), "no-priv")
	if err == nil {
		t.Fatal("expected ErrCodeUnknownKeyID for unregistered kid")
	}
	code, ok := CodeOf(err)
	if !ok || code != ErrCodeUnknownKeyID {
		t.Errorf("expected ErrCodeUnknownKeyID, got %v", err)
	}
}

func TestRemoteJWKSKeyProvider_ActiveKeyID(t *testing.T) {
	priv := generateRSAKey(t)
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{"kid-1": &priv.PublicKey}}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	// No active key yet.
	_, err := kp.ActiveKeyID(context.Background())
	if err == nil {
		t.Fatal("expected error when no active key is set")
	}

	// Register and activate.
	if err := kp.SetPrivateKey("kid-1", priv); err != nil {
		t.Fatalf("SetPrivateKey: %v", err)
	}
	if err := kp.SetActive("kid-1"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	active, err := kp.ActiveKeyID(context.Background())
	if err != nil {
		t.Fatalf("ActiveKeyID: %v", err)
	}
	if active != "kid-1" {
		t.Errorf("expected active kid-1, got %q", active)
	}
}

func TestRemoteJWKSKeyProvider_SetActive_UnregisteredKid(t *testing.T) {
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{}}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	err := kp.SetActive("not-registered")
	if err == nil {
		t.Fatal("expected error when activating kid that has no private key")
	}
}

func TestRemoteJWKSKeyProvider_RevokeKey(t *testing.T) {
	priv := generateRSAKey(t)
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{"kid-1": &priv.PublicKey}}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	_ = kp.SetPrivateKey("kid-1", priv)
	_ = kp.SetActive("kid-1")

	kp.RevokeKey("kid-1")

	// GetPrivateKey should fail after revocation.
	_, err := kp.GetPrivateKey(context.Background(), "kid-1")
	if err == nil {
		t.Fatal("expected error after key revocation")
	}

	// Active should be cleared.
	_, err = kp.ActiveKeyID(context.Background())
	if err == nil {
		t.Fatal("expected error for active kid after revocation")
	}
}

func TestRemoteJWKSKeyProvider_EncryptDecryptRoundTrip(t *testing.T) {
	// Full round-trip: encrypt with RemoteJWKSKeyProvider (public key from
	// fetcher), decrypt with same provider (private key from registry).
	priv := generateRSAKey(t)
	fetcher := &mockFetcher{keys: map[string]*rsa.PublicKey{"enc-v1": &priv.PublicKey}}
	kp := NewRemoteJWKSKeyProvider(fetcher)

	if err := kp.SetPrivateKey("enc-v1", priv); err != nil {
		t.Fatalf("SetPrivateKey: %v", err)
	}
	if err := kp.SetActive("enc-v1"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	enc := NewEncryptor(kp)
	dec := NewDecryptor(kp, WithReplayStore(NewInMemoryReplayStore(), DefaultReplayPolicy))
	ctx := context.Background()

	plaintext := []byte(`{"account":"X999","amount":500000}`)
	meta := Metadata{Method: "POST", Path: "/api/v1/transfer", ContentType: "application/json"}

	envelope, err := enc.Encrypt(ctx, plaintext, meta)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := dec.DecryptEnvelope(ctx, envelope, meta)
	if err != nil {
		t.Fatalf("DecryptEnvelope: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("round-trip mismatch: got %q want %q", got, plaintext)
	}
}

func TestNewRemoteJWKSKeyProvider_NilFetcher_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil fetcher")
		}
	}()
	NewRemoteJWKSKeyProvider(nil)
}
