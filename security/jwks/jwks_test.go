package jwks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

func TestJWKSManager_GetKey(t *testing.T) {
	// 1. Setup RSA Key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := &privateKey.PublicKey

	n := base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes())

	kid := "test-kid"
	jwksResp := Jwks{
		Keys: []Key{
			{
				Kid: kid,
				Kty: "RSA",
				Alg: "RS256",
				N:   n,
				E:   e,
			},
		},
	}

	// 2. Setup Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	// 3. Test Manager
	cfg := Config{
		JWKSURL:        server.URL,
		CacheTTL:       time.Minute,
		RefreshTimeout: time.Second,
	}
	manager := NewJWKSManager(cfg, generateLogger())

	// First call - should fetch
	key, err := manager.GetKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("failed to get key: %v", err)
	}

	if key.N.Cmp(pubKey.N) != 0 || key.E != pubKey.E {
		t.Error("returned key does not match original public key")
	}

	// Unknown kid - should refresh and then fail if still not found
	_, err = manager.GetKey(context.Background(), "unknown-kid")
	if err == nil {
		t.Error("expected error for unknown kid")
	}
}

func generateLogger() logger.Logger {
	l := logger.NewZerologLogger(
		logger.Config{
			Output:              "stdout",
			LogDir:              "",
			LogLevel:            0,
			FilterParams:        nil,
			MaxLengthParamValue: 256,
		},
	)
	return l
}

func TestJWKSManager_ThunderingHerd(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		time.Sleep(100 * time.Millisecond) // Simulate delay
		json.NewEncoder(w).Encode(Jwks{
			Keys: []Key{{Kid: "kid", Kty: "RSA", N: "...", E: "..."}},
		})
	}))
	defer server.Close()

	manager := NewJWKSManager(Config{JWKSURL: server.URL}, generateLogger())

	// Concurrent calls
	const concurrent = 10
	done := make(chan bool)
	for range concurrent {
		go func() {
			manager.GetKey(context.Background(), "kid")
			done <- true
		}()
	}

	for range concurrent {
		<-done
	}

	if callCount != 1 {
		t.Errorf("expected 1 call to JWKS endpoint, got %d", callCount)
	}
}
