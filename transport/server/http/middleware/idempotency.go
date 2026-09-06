package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	appError "github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/gin-gonic/gin"
)

var (
	ErrMissingIdempotencyKey = errors.New("idempotency key is required")
	ErrInflightConflict      = errors.New("concurrent request in progress, please retry")
)

// IdempotencyStorage defines the storage interface for caching idempotent responses.
// Note that SDK's storage/cache.Cache automatically satisfies this interface.
type IdempotencyStorage interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
}

// CanonicalKeyFunc defines the signature for extracting or generating a canonical idempotency key.
type CanonicalKeyFunc func(c *gin.Context, reqBody []byte) (string, error)

// CachedResponse stores HTTP response attributes for idempotent replay.
type CachedResponse struct {
	StatusCode  int               `json:"status_code"`
	ContentType string            `json:"content_type"`
	Body        []byte            `json:"body"`
	Headers     map[string]string `json:"headers,omitempty"`
}

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

// In-flight concurrency tracker
var inflightMap = sync.Map{}

// Idempotency returns a Gin middleware ensuring idempotent request execution.
func Idempotency(storage IdempotencyStorage, opts ...IdempotencyOption) gin.HandlerFunc {
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
		cfg.Storage = NewMemoryStorage()
	}

	return func(c *gin.Context) {
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

		// 1. Check existing cached response
		if cachedData, err := cfg.Storage.Get(c.Request.Context(), storageKey); err == nil && len(cachedData) > 0 {
			if replayCachedResponse(c, cachedData) {
				return
			}
		}

		// 2. Concurrency lock for in-flight requests
		ch := make(chan struct{})
		actual, loaded := inflightMap.LoadOrStore(storageKey, ch)
		if loaded {
			handleInflightConflict(c, cfg, storageKey, actual.(chan struct{}))
			return
		}

		defer func() {
			close(ch)
			inflightMap.Delete(storageKey)
		}()

		// 3. Capture response and execute downstream handlers
		recorder := captureResponseBody(c)
		c.Next()

		// 4. Cache response if eligible
		saveIdempotentResponse(c, cfg.Storage, storageKey, recorder, cfg.TTL)
	}
}

func shouldIgnoreMethod(method string, ignored []string) bool {
	for _, m := range ignored {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

func replayCachedResponse(c *gin.Context, data []byte) bool {
	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return false
	}

	c.Header("X-Cache-Lookup", "HIT")
	c.Header("X-Idempotent-Replayed", "true")
	if cached.ContentType != "" {
		c.Header("Content-Type", cached.ContentType)
	}
	for k, v := range cached.Headers {
		c.Header(k, v)
	}

	c.Status(cached.StatusCode)
	_, _ = c.Writer.Write(cached.Body)
	c.Abort()
	return true
}

func handleInflightConflict(c *gin.Context, cfg IdempotencyConfig, storageKey string, waitCh chan struct{}) {
	select {
	case <-waitCh:
		// Request 1 finished, check storage
		if cachedData, err := cfg.Storage.Get(c.Request.Context(), storageKey); err == nil && len(cachedData) > 0 {
			if replayCachedResponse(c, cachedData) {
				return
			}
		}
		abortIdemError(c, cfg.AppName, http.StatusConflict, ErrInflightConflict.Error())
	case <-c.Request.Context().Done():
		c.AbortWithStatus(http.StatusRequestTimeout)
	case <-time.After(cfg.LockTimeout):
		abortIdemError(c, cfg.AppName, http.StatusConflict, "idempotency lock timeout exceeded")
	}
}

func saveIdempotentResponse(c *gin.Context, storage IdempotencyStorage, storageKey string, recorder *bodyLogWriter, ttl time.Duration) {
	status := c.Writer.Status()
	// Only cache successful or client validation responses (not 5xx server panics/errors)
	if status >= 500 {
		return
	}

	resp := CachedResponse{
		StatusCode:  status,
		ContentType: c.Writer.Header().Get("Content-Type"),
		Body:        recorder.body.Bytes(),
	}

	if data, err := json.Marshal(resp); err == nil {
		_ = storage.Set(c.Request.Context(), storageKey, data, ttl)
	}
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

// MemoryStorage implements in-memory IdempotencyStorage with TTL.
type MemoryStorage struct {
	items sync.Map
}

type memoryStorageItem struct {
	data      []byte
	expiresAt time.Time
}

// NewMemoryStorage creates an in-memory IdempotencyStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

func (m *MemoryStorage) Get(_ context.Context, key string) ([]byte, error) {
	val, ok := m.items.Load(key)
	if !ok {
		return nil, errors.New("key not found")
	}
	item := val.(memoryStorageItem)
	if time.Now().After(item.expiresAt) {
		m.items.Delete(key)
		return nil, errors.New("key expired")
	}
	return item.data, nil
}

func (m *MemoryStorage) Set(_ context.Context, key string, value []byte, expiration time.Duration) error {
	m.items.Store(key, memoryStorageItem{
		data:      value,
		expiresAt: time.Now().Add(expiration),
	})
	return nil
}

func (m *MemoryStorage) Delete(_ context.Context, key string) error {
	m.items.Delete(key)
	return nil
}
