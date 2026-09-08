package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/security/jwks"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestJWTAuthenticator_Success(t *testing.T) {
	// 1. Setup RSA Key and JWKS
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := &privateKey.PublicKey
	kid := "test-key"

	jwksResp := jwks.Jwks{
		Keys: []jwks.Key{
			{
				Kid: kid,
				Kty: "RSA",
				Alg: "RS256",
				N:   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	// 2. Setup Authenticator
	auth, err := New(Config{
		JWKSURL:  server.URL,
		Issuer:   "test-issuer",
		Audience: "test-audience",
	}, logger.Nop())
	if err != nil {
		t.Fatal(err)
	}

	// 3. Create Token

	token, err := Issue(Config{
		Kid:      kid,
		Issuer:   "test-issuer",
		Audience: "test-audience",
	}, privateKey, "USR001", "", time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Authenticate
	res, err := auth.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialBearer,
		Value: token,
	})

	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}

	if res.Principal.Subject != "USR001" {
		t.Errorf("expected principal Subject USR001, got %s", res.Principal.Subject)
	}
}

func TestJWTAuthenticator_InvalidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	auth, _ := New(Config{JWKSURL: server.URL}, logger.Nop())

	_, err := auth.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialBearer,
		Value: "invalid.token.here",
	})

	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestJWTAuthenticator_TracePropagation(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubKey := &privateKey.PublicKey
	kid := "test-trace-key"

	jwksResp := jwks.Jwks{
		Keys: []jwks.Key{
			{
				Kid: kid,
				Kty: "RSA",
				Alg: "RS256",
				N:   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
			},
		},
	}

	var receivedTraceparent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTraceparent = r.Header.Get("Traceparent")
		json.NewEncoder(w).Encode(jwksResp)
	}))
	defer server.Close()

	auth, err := New(Config{
		JWKSURL:  server.URL,
		Issuer:   "test-issuer",
		Audience: "test-audience",
	}, logger.Nop())
	if err != nil {
		t.Fatal(err)
	}

	token, err := Issue(Config{
		Kid:      kid,
		Issuer:   "test-issuer",
		Audience: "test-audience",
	}, privateKey, "USR001", "", time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider()
	tracer := tp.Tracer("test-tracer")
	ctx, span := tracer.Start(context.Background(), "parent-request-operation")
	defer span.End()

	parentTraceID := span.SpanContext().TraceID().String()

	res, err := auth.Authenticate(ctx, security.Credential{
		Type:  security.CredentialBearer,
		Value: token,
	})
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}
	if res.Principal.Subject != "USR001" {
		t.Errorf("expected principal Subject USR001, got %s", res.Principal.Subject)
	}

	if receivedTraceparent == "" {
		t.Fatal("expected Traceparent header in outgoing JWKS request, but got empty")
	}

	if !strings.Contains(receivedTraceparent, parentTraceID) {
		t.Fatalf("expected Traceparent to contain parent trace ID %s, but got %s", parentTraceID, receivedTraceparent)
	}
}
