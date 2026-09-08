package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auth-service/app/config"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	crypto "github.com/agilsyofian/gosdk/utils/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldDecryptor_AuthService_RemoteJWKS(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	const encKid = "enc_v1"
	jwk := crypto.PublicKeyToJWK(&privKey.PublicKey, encKid)
	jwksPayload := crypto.Jwks{Keys: []crypto.Key{jwk}}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksPayload)
	}))
	defer jwksServer.Close()

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	privB64 := base64.StdEncoding.EncodeToString(pkcs8Bytes)

	var cfg config.AppConfig
	cfg.Security.Kid.Encryption = encKid
	cfg.Security.JWKSURL = jwksServer.URL + "/auth/certs"
	cfg.Security.PrivateKey = map[string]string{
		encKid: privB64,
	}
	cfg.Security.Authenticators.Jwt.CacheTTL = time.Hour
	cfg.Security.Authenticators.Jwt.RefreshTimeout = 5 * time.Second

	dec, err := FieldDecryptor(cfg, logger.Nop())
	require.NoError(t, err)
	require.NotNil(t, dec)

	staticKP, err := crypto.NewJWKSKeyProvider(jwksPayload)
	require.NoError(t, err)
	enc := crypto.NewFieldEncryptor(staticKP)

	ctx := context.Background()
	req, err := enc.Encrypt(ctx, encKid, map[string]any{
		"password": "secret-auth-password",
	}, []string{"password"})
	require.NoError(t, err)

	var encryptedPwd string
	err = json.Unmarshal(req.Body["password"], &encryptedPwd)
	require.NoError(t, err)

	decSession, err := dec.Open(ctx, req.Headers)
	require.NoError(t, err)

	decryptedVal, err := decSession.Decrypt(encryptedPwd)
	require.NoError(t, err)
	assert.Equal(t, "secret-auth-password", decryptedVal)
}

func TestFieldDecryptor_AuthService_LocalJWKSFallback(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	const encKid = "enc_v1"
	jwk := crypto.PublicKeyToJWK(&privKey.PublicKey, encKid)
	jwksPayload := crypto.Jwks{Keys: []crypto.Key{jwk}}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	privB64 := base64.StdEncoding.EncodeToString(pkcs8Bytes)

	var cfg config.AppConfig
	cfg.Security.Kid.Encryption = encKid
	cfg.Security.JWKS = jwksPayload // Local JWKS
	cfg.Security.PrivateKey = map[string]string{
		encKid: privB64,
	}

	dec, err := FieldDecryptor(cfg, logger.Nop())
	require.NoError(t, err)
	require.NotNil(t, dec)

	staticKP, err := crypto.NewJWKSKeyProvider(jwksPayload)
	require.NoError(t, err)
	enc := crypto.NewFieldEncryptor(staticKP)

	ctx := context.Background()
	req, err := enc.Encrypt(ctx, encKid, map[string]any{
		"password": "local-fallback-password",
	}, []string{"password"})
	require.NoError(t, err)

	var encryptedPwd string
	err = json.Unmarshal(req.Body["password"], &encryptedPwd)
	require.NoError(t, err)

	decSession, err := dec.Open(ctx, req.Headers)
	require.NoError(t, err)

	decryptedVal, err := decSession.Decrypt(encryptedPwd)
	require.NoError(t, err)
	assert.Equal(t, "local-fallback-password", decryptedVal)
}
