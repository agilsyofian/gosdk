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

	"github.com/agilsyofian/gosdk/core/observability/logger"
	crypto "github.com/agilsyofian/gosdk/utils/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"user-service/app/config"
)

func TestFieldDecryptor_RemoteJWKS(t *testing.T) {
	// 1. Generate RSA key pair for testing
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	const encKid = "enc_v1"
	jwk := crypto.PublicKeyToJWK(&privKey.PublicKey, encKid)
	jwksPayload := crypto.Jwks{Keys: []crypto.Key{jwk}}

	// 2. Mock JWKS HTTP server (simulating auth-service or Keystore)
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksPayload)
	}))
	defer jwksServer.Close()

	// 3. Prepare config for user-service (no static JWKS, only jwks_url + local private key)
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

	// 4. Initialize FieldDecryptor
	dec, err := FieldDecryptor(cfg, logger.Nop())
	require.NoError(t, err)
	require.NotNil(t, dec)

	// 5. Test encrypt with public key and decrypt with FieldDecryptor
	staticKP, err := crypto.NewJWKSKeyProvider(jwksPayload)
	require.NoError(t, err)
	enc := crypto.NewFieldEncryptor(staticKP)

	ctx := context.Background()
	req, err := enc.Encrypt(ctx, encKid, map[string]any{
		"pin": "super-secret-pin-1234",
	}, []string{"pin"})
	require.NoError(t, err)

	var encryptedPin string
	err = json.Unmarshal(req.Body["pin"], &encryptedPin)
	require.NoError(t, err)

	// Decrypt using our RemoteJWKS-backed decryptor
	decSession, err := dec.Open(ctx, req.Headers)
	require.NoError(t, err)

	decryptedVal, err := decSession.Decrypt(encryptedPin)
	require.NoError(t, err)
	assert.Equal(t, "super-secret-pin-1234", decryptedVal)
}
