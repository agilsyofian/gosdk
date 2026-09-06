package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/security/signature"
	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSignatureMiddleware_ValidRequestAndResponseSigning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte("shared-secret-key-12345")
	provider := signature.StaticSecret(secret)

	r := gin.New()
	r.Use(middleware.Signature(provider))
	r.POST("/api/pay", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "paid"})
	})

	body := []byte(`{"order_id":"123","amount":50000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pay", bytes.NewBuffer(body))

	now := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("X-Timestamp", now)

	// Build valid request signature
	canon := signature.NewDefaultCanonicalizer()
	sts, err := canon.BuildStringToSign(req, body, now)
	assert.NoError(t, err)

	sig, err := signature.Sign(signature.AlgorithmHMACSHA256, []byte(sts), secret, signature.EncodingHex)
	assert.NoError(t, err)
	req.Header.Set("X-Signature", sig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"paid"`)

	// Verify response signature headers
	respSig := w.Header().Get("X-Signature")
	respTs := w.Header().Get("X-Timestamp")
	assert.NotEmpty(t, respSig)
	assert.NotEmpty(t, respTs)

	respCanon := signature.NewDefaultResponseCanonicalizer()
	respSTS, err := respCanon.BuildResponseStringToSign(http.StatusOK, w.Body.Bytes(), respTs)
	assert.NoError(t, err)
	assert.True(t, signature.Verify(signature.AlgorithmHMACSHA256, []byte(respSTS), respSig, secret, signature.EncodingHex))
}

func TestSignatureMiddleware_TamperedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte("shared-secret-key-12345")
	provider := signature.StaticSecret(secret)

	r := gin.New()
	r.Use(middleware.Signature(provider))
	r.POST("/api/pay", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "paid"})
	})

	body := []byte(`{"order_id":"123","amount":50000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pay", bytes.NewBuffer([]byte(`{"order_id":"123","amount":9999999}`)))

	now := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("X-Timestamp", now)

	canon := signature.NewDefaultCanonicalizer()
	sts, _ := canon.BuildStringToSign(req, body, now)
	sig, _ := signature.Sign(signature.AlgorithmHMACSHA256, []byte(sts), secret, signature.EncodingHex)
	req.Header.Set("X-Signature", sig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSignatureMiddleware_MissingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := signature.StaticSecret([]byte("sec"))

	r := gin.New()
	r.Use(middleware.Signature(provider))
	r.POST("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Missing both
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Missing timestamp
	req = httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("X-Signature", "some-sig")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSignatureMiddleware_ExpiredTimestamp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte("sec")
	provider := signature.StaticSecret(secret)

	r := gin.New()
	r.Use(middleware.Signature(provider, signature.WithMaxClockDrift(1*time.Minute)))
	r.POST("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	expired := time.Now().Add(-15 * time.Minute).UTC().Format(time.RFC3339)
	req.Header.Set("X-Timestamp", expired)
	req.Header.Set("X-Signature", "sig")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSignatureMiddleware_DisabledResponseSigning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte("sec")
	provider := signature.StaticSecret(secret)

	r := gin.New()
	r.Use(middleware.Signature(provider, signature.WithSignResponse(false)))
	r.POST("/api/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	now := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("X-Timestamp", now)

	canon := signature.NewDefaultCanonicalizer()
	sts, _ := canon.BuildStringToSign(req, nil, now)
	sig, _ := signature.Sign(signature.AlgorithmHMACSHA256, []byte(sts), secret, signature.EncodingHex)
	req.Header.Set("X-Signature", sig)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-Signature"))
}
