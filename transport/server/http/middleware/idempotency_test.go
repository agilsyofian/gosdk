package middleware_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestIdempotency_CustomCanonicalKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := middleware.NewMemoryStorage()

	var executionCount atomic.Int32

	// Custom canonical key: consumer extracts specific transaction ID from body or custom header
	customKeyFunc := func(c *gin.Context, body []byte) (string, error) {
		var payload struct {
			TransactionID string `json:"transaction_id"`
		}
		if err := json.Unmarshal(body, &payload); err == nil && payload.TransactionID != "" {
			return "custom:txn:" + payload.TransactionID, nil
		}
		return "", fmt.Errorf("transaction_id is required")
	}

	r := gin.New()
	r.POST("/payments",
		middleware.Idempotency(storage,
			middleware.WithCanonicalKeyFunc(customKeyFunc),
			middleware.WithTTL(10*time.Minute),
		),
		func(c *gin.Context) {
			count := executionCount.Add(1)
			c.JSON(http.StatusCreated, gin.H{
				"status": "payment_processed",
				"exec":   count,
			})
		},
	)

	body := `{"transaction_id": "TX-998822", "amount": 50000}`

	// First Request -> Handler executes
	req1 := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusCreated, w1.Code)
	assert.Equal(t, int32(1), executionCount.Load())
	assert.Empty(t, w1.Header().Get("X-Idempotent-Replayed"))

	// Second Request with identical transaction_id -> Replayed from cache, handler NOT executed
	req2 := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusCreated, w2.Code)
	assert.Equal(t, int32(1), executionCount.Load(), "Handler should not run second time")
	assert.Equal(t, "HIT", w2.Header().Get("X-Cache-Lookup"))
	assert.Equal(t, "true", w2.Header().Get("X-Idempotent-Replayed"))
	assert.Equal(t, w1.Body.String(), w2.Body.String())

	// Third Request with different transaction_id -> Handler executes again
	bodyDifferent := `{"transaction_id": "TX-998833", "amount": 75000}`
	req3 := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(bodyDifferent))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	assert.Equal(t, http.StatusCreated, w3.Code)
	assert.Equal(t, int32(2), executionCount.Load())
}

func TestIdempotency_HeaderCanonicalKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := middleware.NewMemoryStorage()

	var count atomic.Int32

	r := gin.New()
	r.POST("/orders",
		middleware.Idempotency(storage,
			middleware.WithHeader("X-Idempotency-Key"),
			middleware.WithTTL(5*time.Second),
		),
		func(c *gin.Context) {
			val := count.Add(1)
			c.JSON(http.StatusOK, gin.H{"order_id": 1001, "call": val})
		},
	)

	// Missing header -> Bad Request (400)
	reqNoHeader := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"item": "book"}`))
	wNoHeader := httptest.NewRecorder()
	r.ServeHTTP(wNoHeader, reqNoHeader)
	assert.Equal(t, http.StatusBadRequest, wNoHeader.Code)

	// Request with header -> Success
	req1 := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"item": "book"}`))
	req1.Header.Set("X-Idempotency-Key", "ORDER-KEY-ABC")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, int32(1), count.Load())

	// Replay with same header -> Replay cached response
	req2 := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"item": "book"}`))
	req2.Header.Set("X-Idempotency-Key", "ORDER-KEY-ABC")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, int32(1), count.Load())
	assert.Equal(t, "true", w2.Header().Get("X-Idempotent-Replayed"))
}

func TestIdempotency_DefaultCanonicalKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := middleware.NewMemoryStorage()

	var count atomic.Int32

	r := gin.New()
	r.POST("/transfers",
		middleware.Idempotency(storage), // Default canonical key
		func(c *gin.Context) {
			count.Add(1)
			c.JSON(http.StatusOK, gin.H{"transferred": true})
		},
	)

	body := `{"to": "account-123", "amount": 1000}`

	// Device 1 transfer
	req1 := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(body))
	req1.Header.Set("X-Device-Id", "device-A")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, int32(1), count.Load())

	// Device 1 retry with same body -> Cached
	req2 := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(body))
	req2.Header.Set("X-Device-Id", "device-A")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, int32(1), count.Load())
	assert.Equal(t, "true", w2.Header().Get("X-Idempotent-Replayed"))

	// Device 2 with same body -> Executed separately (different device canonical key)
	req3 := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(body))
	req3.Header.Set("X-Device-Id", "device-B")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Equal(t, int32(2), count.Load())
}

func TestIdempotency_ConcurrentInflightLock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := middleware.NewMemoryStorage()

	var handlerExecuted atomic.Int32

	r := gin.New()
	r.POST("/heavy-computation",
		middleware.Idempotency(storage,
			middleware.WithSimpleKeyFunc(func(c *gin.Context, body []byte) string {
				return "fixed-concurrent-key"
			}),
			middleware.WithLockTimeout(5*time.Second),
		),
		func(c *gin.Context) {
			handlerExecuted.Add(1)
			// Simulate slow processing
			time.Sleep(100 * time.Millisecond)
			c.JSON(http.StatusOK, gin.H{"result": 42})
		},
	)

	var wg sync.WaitGroup
	numRequests := 5
	recorders := make([]*httptest.ResponseRecorder, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		recorders[i] = httptest.NewRecorder()
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/heavy-computation", bytes.NewBufferString(`{}`))
			r.ServeHTTP(recorders[idx], req)
		}(i)
	}

	wg.Wait()

	// Only 1 execution occurred!
	assert.Equal(t, int32(1), handlerExecuted.Load(), "Handler must only execute once despite concurrent calls")

	// All 5 requests should receive 200 OK with identical body
	for i := 0; i < numRequests; i++ {
		assert.Equal(t, http.StatusOK, recorders[i].Code)
		assert.Contains(t, recorders[i].Body.String(), `"result":42`)
	}
}

func TestIdempotency_IgnoreMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := middleware.NewMemoryStorage()

	var count atomic.Int32

	r := gin.New()
	r.GET("/items",
		middleware.Idempotency(storage),
		func(c *gin.Context) {
			count.Add(1)
			c.String(http.StatusOK, "list")
		},
	)

	// GET requests should bypass idempotency and always execute
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/items", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	assert.Equal(t, int32(3), count.Load())
}
