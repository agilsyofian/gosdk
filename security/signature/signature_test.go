package signature

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSign_Verify_HMACSHA256(t *testing.T) {
	key := []byte("my-secret-key-12345")
	payload := []byte("POST\n/api/v1/payment\n2026-09-06T10:00:00Z\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")

	// Hex
	sigHex, err := Sign(AlgorithmHMACSHA256, payload, key, EncodingHex)
	if err != nil {
		t.Fatalf("sign hex failed: %v", err)
	}
	if !Verify(AlgorithmHMACSHA256, payload, sigHex, key, EncodingHex) {
		t.Fatalf("verify hex failed")
	}

	// Tampered payload
	if Verify(AlgorithmHMACSHA256, []byte("tampered"), sigHex, key, EncodingHex) {
		t.Fatalf("verify should fail on tampered payload")
	}

	// Base64
	sigB64, err := Sign(AlgorithmHMACSHA256, payload, key, EncodingBase64)
	if err != nil {
		t.Fatalf("sign base64 failed: %v", err)
	}
	if !Verify(AlgorithmHMACSHA256, payload, sigB64, key, EncodingBase64) {
		t.Fatalf("verify base64 failed")
	}
}

func TestSign_Verify_HMACSHA512(t *testing.T) {
	key := []byte("my-512-secret-key")
	payload := []byte("POST:/v1/transfer:bodyhash:timestamp")

	sig, err := Sign(AlgorithmHMACSHA512, payload, key, EncodingHex)
	if err != nil {
		t.Fatalf("sign hmac-sha512 failed: %v", err)
	}
	if !Verify(AlgorithmHMACSHA512, payload, sig, key, EncodingHex) {
		t.Fatalf("verify hmac-sha512 failed")
	}
}

func TestEngine_VerifyRequest_Success(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret-123")
	provider := StaticSecret(secret)
	engine := NewEngine(provider, WithMaxClockDrift(5*time.Minute))

	body := []byte(`{"amount":100000}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/payment", bytes.NewBuffer(body))

	now := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("X-Timestamp", now)

	stringToSign, err := engine.Config().Canonicalizer.BuildStringToSign(req, body, now)
	if err != nil {
		t.Fatalf("failed to build string to sign: %v", err)
	}

	sig, err := Sign(AlgorithmHMACSHA256, []byte(stringToSign), secret, EncodingHex)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	req.Header.Set("X-Signature", sig)

	keyID, sec, err := engine.VerifyRequest(ctx, req, body)
	if err != nil {
		t.Fatalf("verify request failed: %v", err)
	}
	if string(sec) != string(secret) {
		t.Fatalf("expected secret %s, got %s", secret, sec)
	}
	_ = keyID
}

func TestEngine_VerifyRequest_TamperedBody(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret-123")
	engine := NewEngine(StaticSecret(secret))

	body := []byte(`{"amount":100000}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/payment", bytes.NewBuffer(body))
	now := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("X-Timestamp", now)

	stringToSign, _ := engine.Config().Canonicalizer.BuildStringToSign(req, body, now)
	sig, _ := Sign(AlgorithmHMACSHA256, []byte(stringToSign), secret, EncodingHex)
	req.Header.Set("X-Signature", sig)

	// Tamper body
	tamperedBody := []byte(`{"amount":999999999}`)
	_, _, err := engine.VerifyRequest(ctx, req, tamperedBody)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestEngine_VerifyRequest_ExpiredTimestamp(t *testing.T) {
	ctx := context.Background()
	secret := []byte("test-secret-123")
	engine := NewEngine(StaticSecret(secret), WithMaxClockDrift(1*time.Minute))

	body := []byte(`{"ping":true}`)
	req := httptest.NewRequest(http.MethodPost, "/ping", bytes.NewBuffer(body))

	expiredTime := time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
	req.Header.Set("X-Timestamp", expiredTime)
	req.Header.Set("X-Signature", "some-sig")

	_, _, err := engine.VerifyRequest(ctx, req, body)
	if !errors.Is(err, ErrTimestampExpired) {
		t.Fatalf("expected ErrTimestampExpired, got %v", err)
	}
}

func TestEngine_VerifyRequest_MissingHeaders(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(StaticSecret([]byte("sec")))
	req := httptest.NewRequest(http.MethodPost, "/test", nil)

	_, _, err := engine.VerifyRequest(ctx, req, nil)
	if !errors.Is(err, ErrMissingSignature) {
		t.Fatalf("expected ErrMissingSignature, got %v", err)
	}

	req.Header.Set("X-Signature", "sig")
	_, _, err = engine.VerifyRequest(ctx, req, nil)
	if !errors.Is(err, ErrMissingTimestamp) {
		t.Fatalf("expected ErrMissingTimestamp, got %v", err)
	}
}

func TestEngine_SignResponse(t *testing.T) {
	secret := []byte("resp-secret-999")
	engine := NewEngine(StaticSecret(secret))

	body := []byte(`{"status":"success"}`)
	sig, ts, err := engine.SignResponse(200, body, secret)
	if err != nil {
		t.Fatalf("sign response failed: %v", err)
	}
	if sig == "" || ts == "" {
		t.Fatalf("expected non-empty sig and ts")
	}

	// Verify the response signature
	stringToSign, err := engine.Config().ResponseCanonicalizer.BuildResponseStringToSign(200, body, ts)
	if err != nil {
		t.Fatalf("build response string to sign failed: %v", err)
	}

	if !Verify(AlgorithmHMACSHA256, []byte(stringToSign), sig, secret, EncodingHex) {
		t.Fatalf("expected response signature to verify")
	}
}

func TestSecretProvider_MapAndFunc(t *testing.T) {
	ctx := context.Background()

	// Map
	mp := MapSecret(map[string][]byte{
		"client-A": []byte("sec-A"),
	})
	sec, err := mp.GetSecret(ctx, "client-A")
	if err != nil || string(sec) != "sec-A" {
		t.Fatalf("failed map get secret: %v, %s", err, sec)
	}
	_, err = mp.GetSecret(ctx, "unknown")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}

	// Func
	fn := FuncSecret(func(ctx context.Context, keyID string) ([]byte, error) {
		if keyID == "dynamic" {
			return []byte("dyn-sec"), nil
		}
		return nil, ErrSecretNotFound
	})
	sec, err = fn.GetSecret(ctx, "dynamic")
	if err != nil || string(sec) != "dyn-sec" {
		t.Fatalf("failed func get secret: %v, %s", err, sec)
	}
}
