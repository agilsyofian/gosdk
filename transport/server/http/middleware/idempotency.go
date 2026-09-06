package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	appError "github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/transport/idempotency"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/gin-gonic/gin"
)

var (
	ErrMissingIdempotencyKey = errors.New("idempotency key is required")
	ErrInflightConflict      = errors.New("concurrent request in progress, please retry")
)

// IdempotencyStorage is an alias to standard transport/idempotency.Storage for backward compatibility.
type IdempotencyStorage = idempotency.Storage

// MemoryStorage is an alias to transport/idempotency.MemoryStorage for backward compatibility.
type MemoryStorage = idempotency.MemoryStorage

// NewMemoryStorage creates an in-memory IdempotencyStorage.
func NewMemoryStorage() *MemoryStorage {
	return idempotency.NewMemoryStorage()
}

// CanonicalKeyFunc defines the signature for extracting or generating a canonical idempotency key.
type CanonicalKeyFunc func(c *gin.Context, reqBody []byte) (string, error)

// CachedResponse stores HTTP response attributes for idempotent replay.
type CachedResponse = idempotency.Record

// IdempotencyConfig configures the idempotency middleware.
type IdempotencyConfig struct {
	Storage       IdempotencyStorage
	KeyFunc       CanonicalKeyFunc
	TTL           time.Duration
	LockTimeout   time.Duration
	HeaderName    string
	AppName       string
	IgnoreMethods []string
}

// IdempotencyOption provides functional options to configure Idempotency middleware.
type IdempotencyOption func(*IdempotencyConfig)

// WithTTL sets custom TTL for cached idempotent responses (default: 30s).
func WithTTL(ttl time.Duration) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if ttl > 0 {
			c.TTL = ttl
		}
	}
}

// WithLockTimeout sets maximum wait time for in-flight requests (default: 10s).
func WithLockTimeout(timeout time.Duration) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if timeout > 0 {
			c.LockTimeout = timeout
		}
	}
}

// WithCanonicalKeyFunc sets custom canonical key generation logic.
func WithCanonicalKeyFunc(fn CanonicalKeyFunc) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if fn != nil {
			c.KeyFunc = fn
		}
	}
}

// WithSimpleKeyFunc sets custom canonical key generation using a simpler string return signature.
func WithSimpleKeyFunc(fn func(c *gin.Context, reqBody []byte) string) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if fn != nil {
			c.KeyFunc = func(ctx *gin.Context, body []byte) (string, error) {
				key := fn(ctx, body)
				if key == "" {
					return "", ErrMissingIdempotencyKey
				}
				return key, nil
			}
		}
	}
}

// WithHeader sets an HTTP header to extract the idempotency key (e.g. "X-Idempotency-Key").
func WithHeader(headerName string, fallback ...CanonicalKeyFunc) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		c.HeaderName = headerName
		var fallbackFn CanonicalKeyFunc
		if len(fallback) > 0 {
			fallbackFn = fallback[0]
		}
		c.KeyFunc = HeaderCanonicalKey(headerName, fallbackFn)
	}
}

// WithAppName sets application name for structured error responses.
func WithAppName(appName string) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		c.AppName = appName
	}
}

// WithIgnoreMethods specifies HTTP methods that bypass idempotency checking (e.g. "GET", "HEAD", "OPTIONS").
func WithIgnoreMethods(methods ...string) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		c.IgnoreMethods = methods
	}
}

// DefaultCanonicalKey generates a canonical idempotency key using client identity, HTTP method, path, and body hash.
func DefaultCanonicalKey(c *gin.Context, body []byte) (string, error) {
	clientID := extractClientIdentifier(c)
	method := c.Request.Method
	path := c.Request.URL.Path

	bodyHash := sha256.Sum256(body)
	raw := fmt.Sprintf("%s|%s|%s|%x", clientID, method, path, bodyHash)

	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:]), nil
}

// HeaderCanonicalKey extracts the idempotency key from a specified request header, with optional fallback.
func HeaderCanonicalKey(headerName string, fallback CanonicalKeyFunc) CanonicalKeyFunc {
	return func(c *gin.Context, body []byte) (string, error) {
		val := strings.TrimSpace(c.GetHeader(headerName))
		if val != "" {
			return val, nil
		}
		if fallback != nil {
			return fallback(c, body)
		}
		return "", fmt.Errorf("%w: %s", ErrMissingIdempotencyKey, headerName)
	}
}

func extractClientIdentifier(c *gin.Context) string {
	for _, h := range []string{"X-Device-Id", "Device-Id", "X-User-Id", "User-Id"} {
		if val := strings.TrimSpace(c.GetHeader(h)); val != "" {
			return val
		}
	}
	return c.ClientIP()
}

// Idempotency returns a Gin middleware ensuring idempotent request execution backed by transport/idempotency.Engine.
func Idempotency(storage IdempotencyStorage, opts ...IdempotencyOption) gin.HandlerFunc {
	cfg := resolveConfig(storage, opts...)
	engine := idempotency.NewEngine(cfg.Storage,
		idempotency.WithDefaultTTL(cfg.TTL),
		idempotency.WithLockTimeout(cfg.LockTimeout),
	)

	return func(c *gin.Context) {
		handleIdempotentRequest(c, engine, cfg)
	}
}

func resolveConfig(storage IdempotencyStorage, opts ...IdempotencyOption) IdempotencyConfig {
	cfg := IdempotencyConfig{
		Storage:       storage,
		KeyFunc:       DefaultCanonicalKey,
		TTL:           30 * time.Second,
		LockTimeout:   10 * time.Second,
		AppName:       "server",
		IgnoreMethods: []string{http.MethodGet, http.MethodHead, http.MethodOptions},
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.Storage == nil {
		cfg.Storage = idempotency.NewMemoryStorage()
	}

	return cfg
}

func handleIdempotentRequest(c *gin.Context, engine *idempotency.Engine, cfg IdempotencyConfig) {
	if shouldIgnoreMethod(c.Request.Method, cfg.IgnoreMethods) {
		c.Next()
		return
	}

	reqBody := captureRequestBody(c)
	idemKey, err := cfg.KeyFunc(c, reqBody)
	if err != nil {
		abortIdemError(c, cfg.AppName, http.StatusBadRequest, err.Error())
		return
	}

	storageKey := "IDEM:" + idemKey
	rec, isReplay, execErr := engine.Execute(c.Request.Context(), storageKey, cfg.TTL, func(_ context.Context) (*idempotency.Record, error) {
		return executeAndRecord(c)
	})

	if execErr != nil {
		handleExecutionError(c, cfg.AppName, execErr)
		return
	}

	if isReplay && rec != nil {
		replayCachedResponse(c, rec)
	}
}

func executeAndRecord(c *gin.Context) (*idempotency.Record, error) {
	recorder := captureResponseBody(c)
	c.Next()

	status := c.Writer.Status()
	if status >= 500 {
		return nil, nil // Do not cache 5xx server panics/errors
	}

	headers := make(map[string]string)
	if ct := c.Writer.Header().Get("Content-Type"); ct != "" {
		headers["Content-Type"] = ct
	}

	return &idempotency.Record{
		StatusCode: status,
		Headers:    headers,
		Body:       recorder.body.Bytes(),
	}, nil
}

func handleExecutionError(c *gin.Context, appName string, err error) {
	if errors.Is(err, idempotency.ErrTimeoutWaitingInflight) {
		abortIdemError(c, appName, http.StatusConflict, "idempotency lock timeout exceeded")
		return
	}
	if errors.Is(err, idempotency.ErrInflightFailed) {
		abortIdemError(c, appName, http.StatusConflict, ErrInflightConflict.Error())
		return
	}
	abortIdemError(c, appName, http.StatusConflict, err.Error())
}

func shouldIgnoreMethod(method string, ignored []string) bool {
	for _, m := range ignored {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

func replayCachedResponse(c *gin.Context, rec *idempotency.Record) {
	c.Header("X-Cache-Lookup", "HIT")
	c.Header("X-Idempotent-Replayed", "true")
	for k, v := range rec.Headers {
		c.Header(k, v)
	}

	c.Status(rec.StatusCode)
	_, _ = c.Writer.Write(rec.Body)
	c.Abort()
}

func abortIdemError(c *gin.Context, appName string, status int, message string) {
	appErr := appError.New(
		errors.New(message),
		mapHTTPStatus(status),
		appName,
		fmt.Sprintf("%d", status),
		message,
		nil,
	)
	_ = c.Error(appErr)
	c.AbortWithStatusJSON(status, dto.New(c.Request.Context(), nil).
		WithSource(appName).
		WithCode(fmt.Sprintf("%d", status)).
		WithMessage(message))
}

func mapHTTPStatus(status int) appError.HTTPStatus {
	switch status {
	case http.StatusBadRequest:
		return appError.StatusBadRequest
	case http.StatusConflict:
		return appError.StatusConflict
	case http.StatusRequestTimeout:
		return appError.StatusRequestTimeout
	default:
		return appError.StatusInternalServerError
	}
}
