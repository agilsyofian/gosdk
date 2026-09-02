package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIssue(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Kid:      "test-kid",
		Issuer:   "test-issuer",
		Audience: "test-audience",
	}

	t.Run("success", func(t *testing.T) {
		subject := "user123"
		sessionID := "sess456"
		ttl := time.Hour
		extraClaims := map[string]any{
			"custom": "value",
		}

		token, err := Issue(cfg, privateKey, subject, sessionID, ttl, extraClaims)
		if err != nil {
			t.Fatalf("Issue failed: %v", err)
		}

		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatalf("expected 3 parts in JWT, got %d", len(parts))
		}

		// Verify Header
		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil {
			t.Fatal(err)
		}
		var header map[string]any
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			t.Fatal(err)
		}

		if header["alg"] != "RS256" {
			t.Errorf("expected alg RS256, got %v", header["alg"])
		}
		if header["kid"] != cfg.Kid {
			t.Errorf("expected kid %s, got %v", cfg.Kid, header["kid"])
		}

		// Verify Claims
		claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatal(err)
		}
		var claims map[string]any
		if err := json.Unmarshal(claimsBytes, &claims); err != nil {
			t.Fatal(err)
		}

		if claims["sub"] != subject {
			t.Errorf("expected sub %s, got %v", subject, claims["sub"])
		}
		if claims["sid"] != sessionID {
			t.Errorf("expected sid %s, got %v", sessionID, claims["sid"])
		}
		if claims["iss"] != cfg.Issuer {
			t.Errorf("expected iss %s, got %v", cfg.Issuer, claims["iss"])
		}
		if claims["aud"] != cfg.Audience {
			t.Errorf("expected aud %s, got %v", cfg.Audience, claims["aud"])
		}
		if claims["custom"] != "value" {
			t.Errorf("expected custom claim value, got %v", claims["custom"])
		}
		if _, ok := claims["iat"]; !ok {
			t.Error("expected iat claim to be present")
		}
		if _, ok := claims["exp"]; !ok {
			t.Error("expected exp claim to be present")
		}
	})

	t.Run("reserved claims protection", func(t *testing.T) {

		pvk, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}

		extraClaims := map[string]any{
			"sub": "evil-sub",
			"iss": "evil-iss",
		}
		token, err := Issue(cfg, pvk, "good-sub", "", time.Hour, extraClaims)
		if err != nil {
			t.Fatal(err)
		}

		parts := strings.Split(token, ".")
		claimsBytes, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims map[string]any
		json.Unmarshal(claimsBytes, &claims)

		if claims["sub"] != "good-sub" {
			t.Errorf("reserved claim 'sub' was overridden: %v", claims["sub"])
		}
		if claims["iss"] != cfg.Issuer {
			t.Errorf("reserved claim 'iss' was overridden: %v", claims["iss"])
		}
	})

	t.Run("success with base64 private key", func(t *testing.T) {

		pvk, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}

		token, err := Issue(cfg, pvk, "user123", "sess456", time.Hour, nil)
		if err != nil {
			t.Fatalf("Issue with base64 key failed: %v", err)
		}

		if !strings.Contains(token, ".") {
			t.Error("expected a valid JWT token")
		}
	})
}
