package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	securityCore "github.com/agilsyofian/gosdk/security"
	httpServer "github.com/agilsyofian/gosdk/transport/server/http"
	httpSecurity "github.com/agilsyofian/gosdk/transport/server/http/security"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxtest"
)

func TestServer_Actuator_HealthProbes(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()
	reg := health.NewRegistry()

	cfg := httpServer.Config{
		Addr:            ":0",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Mode:            "test",
		Actuator: httpServer.ActuatorConfig{
			Enabled:    true,
			Path:       "/actuator",
			HealthPath: "/health",
			InfoPath:   "/info",
		},
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, reg)

	// 1. Initial State: All UP
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/actuator/health", nil)
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	wLive := httptest.NewRecorder()
	reqLive, _ := http.NewRequest(http.MethodGet, "/actuator/liveness", nil)
	s.ServeHTTP(wLive, reqLive)
	assert.Equal(t, http.StatusOK, wLive.Code)

	wReady := httptest.NewRecorder()
	reqReady, _ := http.NewRequest(http.MethodGet, "/actuator/readiness", nil)
	s.ServeHTTP(wReady, reqReady)
	assert.Equal(t, http.StatusOK, wReady.Code)

	// 2. Register a Failing Readiness Check (e.g. database down)
	reg.RegisterReadinessFunc("database", func(ctx context.Context) health.Health {
		return health.DownWithDetails(map[string]any{"error": "connection failed"})
	})

	// Liveness must STILL BE 200 OK (Kubernetes should not restart container)
	wLive2 := httptest.NewRecorder()
	s.ServeHTTP(wLive2, reqLive)
	assert.Equal(t, http.StatusOK, wLive2.Code)

	// Readiness must BE 503 Service Unavailable (Kubernetes will remove from service endpoints)
	wReady2 := httptest.NewRecorder()
	s.ServeHTTP(wReady2, reqReady)
	assert.Equal(t, http.StatusServiceUnavailable, wReady2.Code)

	var readyResp health.Health
	err := json.Unmarshal(wReady2.Body.Bytes(), &readyResp)
	assert.NoError(t, err)
	assert.Equal(t, health.StatusDown, readyResp.Status)
	assert.Contains(t, readyResp.Details, "database")

	// Overall health must BE 503
	wHealth2 := httptest.NewRecorder()
	s.ServeHTTP(wHealth2, req)
	assert.Equal(t, http.StatusServiceUnavailable, wHealth2.Code)

	// Info endpoint must be 200 OK
	wInfo := httptest.NewRecorder()
	reqInfo, _ := http.NewRequest(http.MethodGet, "/actuator/info", nil)
	s.ServeHTTP(wInfo, reqInfo)
	assert.Equal(t, http.StatusOK, wInfo.Code)
}

func TestServer_Actuator_NilHealthRegistryFallback(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr: ":0",
		Mode: "test",
		Actuator: httpServer.ActuatorConfig{
			Enabled:    true,
			Path:       "/actuator",
			HealthPath: "/health",
		},
	}

	// hr is passed as nil
	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	assert.NotNil(t, s.HealthRegistry())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/actuator/health", nil)
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_BasePath(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr:     ":0",
		Mode:     "test",
		BasePath: "/api/v1",
		Actuator: httpServer.ActuatorConfig{
			Enabled:    true,
			Path:       "/actuator",
			HealthPath: "/health",
		},
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	assert.Equal(t, "/api/v1", s.BasePath())
	assert.NotNil(t, s.BaseRouter())

	// 1. Direct handler on s (embedded gin.IRouter)
	s.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// 2. Group under Router() / BaseRouter()
	users := s.Router().Group("/users")
	users.GET("/list", func(c *gin.Context) {
		c.String(http.StatusOK, "user-list")
	})

	// 3. Raw route on root Engine (bypassing BasePath)
	s.Engine().GET("/raw", func(c *gin.Context) {
		c.String(http.StatusOK, "raw-response")
	})

	// Test GET /api/v1/ping -> 200 OK
	wPing := httptest.NewRecorder()
	reqPing, _ := http.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	s.ServeHTTP(wPing, reqPing)
	assert.Equal(t, http.StatusOK, wPing.Code)
	assert.Equal(t, "pong", wPing.Body.String())

	// Test GET /ping (without base path) -> 404 Not Found
	wPingRoot := httptest.NewRecorder()
	reqPingRoot, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	s.ServeHTTP(wPingRoot, reqPingRoot)
	assert.Equal(t, http.StatusNotFound, wPingRoot.Code)

	// Test GET /api/v1/users/list -> 200 OK
	wUsers := httptest.NewRecorder()
	reqUsers, _ := http.NewRequest(http.MethodGet, "/api/v1/users/list", nil)
	s.ServeHTTP(wUsers, reqUsers)
	assert.Equal(t, http.StatusOK, wUsers.Code)
	assert.Equal(t, "user-list", wUsers.Body.String())

	// Test GET /raw (outside base path) -> 200 OK
	wRaw := httptest.NewRecorder()
	reqRaw, _ := http.NewRequest(http.MethodGet, "/raw", nil)
	s.ServeHTTP(wRaw, reqRaw)
	assert.Equal(t, http.StatusOK, wRaw.Code)
	assert.Equal(t, "raw-response", wRaw.Body.String())

	// Test Actuator under BasePath -> 200 OK
	wAct := httptest.NewRecorder()
	reqAct, _ := http.NewRequest(http.MethodGet, "/api/v1/actuator/health", nil)
	s.ServeHTTP(wAct, reqAct)
	assert.Equal(t, http.StatusOK, wAct.Code)
}

func TestServer_BasePath_Normalization(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	tests := []struct {
		input    string
		expected string
	}{
		{input: "", expected: ""},
		{input: "/", expected: ""},
		{input: "   ", expected: ""},
		{input: "api/v1", expected: "/api/v1"},
		{input: "/api/v1", expected: "/api/v1"},
		{input: "/api/v1/", expected: "/api/v1"},
		{input: "///api/v1///", expected: "/api/v1"},
		{input: "/api/v2/service/", expected: "/api/v2/service"},
	}

	for _, tt := range tests {
		cfg := httpServer.Config{
			Addr:     ":0",
			Mode:     "test",
			BasePath: tt.input,
		}
		s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
		assert.Equal(t, tt.expected, s.BasePath(), "for input: %s", tt.input)
	}
}

func TestServer_GracefulShutdown_HealthProbes503(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr:            ":0",
		Mode:            "test",
		ShutdownTimeout: 2 * time.Second,
		Actuator: httpServer.ActuatorConfig{
			Enabled:    true,
			Path:       "/actuator",
			HealthPath: "/health",
		},
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	assert.False(t, s.InGracefulShutdown())

	// 1. Before shutdown: Health is 200 OK
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/actuator/health", nil)
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Trigger Shutdown
	err := s.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.True(t, s.InGracefulShutdown())

	// 3. After shutdown: Health returns 503 Service Unavailable with DOWN
	w2 := httptest.NewRecorder()
	s.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)

	var resp health.Health
	err = json.Unmarshal(w2.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, health.StatusDown, resp.Status)
	assert.Equal(t, "server is shutting down", resp.Details["message"])

	// Liveness probe also returns 503
	wLive := httptest.NewRecorder()
	reqLive, _ := http.NewRequest(http.MethodGet, "/actuator/liveness", nil)
	s.ServeHTTP(wLive, reqLive)
	assert.Equal(t, http.StatusServiceUnavailable, wLive.Code)

	// Readiness probe also returns 503
	wReady := httptest.NewRecorder()
	reqReady, _ := http.NewRequest(http.MethodGet, "/actuator/readiness", nil)
	s.ServeHTTP(wReady, reqReady)
	assert.Equal(t, http.StatusServiceUnavailable, wReady.Code)
}

func TestServer_CORS(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr:       ":0",
		Mode:       "test",
		EnableCors: true,
		Cors: httpServer.CorsConfig{
			AllowOrigins:     []string{"https://app.example.com"},
			AllowMethods:     []string{"GET", "POST"},
			AllowCredentials: true,
		},
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	s.GET("/test-cors", func(c *gin.Context) {
		c.String(http.StatusOK, "cors ok")
	})

	// Preflight OPTIONS
	reqOpt, _ := http.NewRequest(http.MethodOptions, "/test-cors", nil)
	reqOpt.Header.Set("Origin", "https://app.example.com")
	reqOpt.Header.Set("Access-Control-Request-Method", "POST")
	wOpt := httptest.NewRecorder()
	s.ServeHTTP(wOpt, reqOpt)

	assert.Equal(t, http.StatusNoContent, wOpt.Code)
	assert.Equal(t, "https://app.example.com", wOpt.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", wOpt.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, wOpt.Header().Get("Vary"), "Origin")

	// Standard GET
	reqGet, _ := http.NewRequest(http.MethodGet, "/test-cors", nil)
	reqGet.Header.Set("Origin", "https://app.example.com")
	wGet := httptest.NewRecorder()
	s.ServeHTTP(wGet, reqGet)

	assert.Equal(t, http.StatusOK, wGet.Code)
	assert.Equal(t, "https://app.example.com", wGet.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, wGet.Header().Get("Vary"), "Origin")
}

func TestServer_BlockMethodOverride(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr: ":0",
		Mode: "test",
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	s.POST("/items", func(c *gin.Context) {
		c.String(http.StatusOK, "created")
	})

	// 1. Normal request passes
	reqOK, _ := http.NewRequest(http.MethodPost, "/items", nil)
	wOK := httptest.NewRecorder()
	s.ServeHTTP(wOK, reqOK)
	assert.Equal(t, http.StatusOK, wOK.Code)

	// 2. Request with X-HTTP-Method-Override is blocked (403)
	reqBlockHeader, _ := http.NewRequest(http.MethodPost, "/items", nil)
	reqBlockHeader.Header.Set("X-HTTP-Method-Override", "DELETE")
	wBlockHeader := httptest.NewRecorder()
	s.ServeHTTP(wBlockHeader, reqBlockHeader)
	assert.Equal(t, http.StatusForbidden, wBlockHeader.Code)

	// 3. Request with _method query param is blocked (403)
	reqBlockQuery, _ := http.NewRequest(http.MethodPost, "/items?_method=PUT", nil)
	wBlockQuery := httptest.NewRecorder()
	s.ServeHTTP(wBlockQuery, reqBlockQuery)
	assert.Equal(t, http.StatusForbidden, wBlockQuery.Code)

	// 4. When disabled, request passes
	cfgDisabled := httpServer.Config{
		Addr:                       ":0",
		Mode:                       "test",
		DisableBlockMethodOverride: true,
	}
	sDisabled := httpServer.NewServer("test-app", lc, l, cfgDisabled, nil, nil)
	sDisabled.POST("/items", func(c *gin.Context) {
		c.String(http.StatusOK, "created")
	})

	reqAllowed, _ := http.NewRequest(http.MethodPost, "/items", nil)
	reqAllowed.Header.Set("X-HTTP-Method-Override", "DELETE")
	wAllowed := httptest.NewRecorder()
	sDisabled.ServeHTTP(wAllowed, reqAllowed)
	assert.Equal(t, http.StatusOK, wAllowed.Code)
}

func TestServer_BlockJSONSpecialChars(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr: ":0",
		Mode: "test",
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	s.POST("/comment", func(c *gin.Context) {
		c.String(http.StatusOK, "saved")
	})

	// 1. Normal JSON passes
	reqOK, _ := http.NewRequest(http.MethodPost, "/comment", bytes.NewBufferString(`{"text": "Hello world"}`))
	reqOK.Header.Set("Content-Type", "application/json")
	wOK := httptest.NewRecorder()
	s.ServeHTTP(wOK, reqOK)
	assert.Equal(t, http.StatusOK, wOK.Code)

	// 2. Dangerous XSS JSON is blocked (400)
	reqBad, _ := http.NewRequest(http.MethodPost, "/comment", bytes.NewBufferString(`{"text": "<script>alert(1)</script>"}`))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	s.ServeHTTP(wBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)

	// 3. When disabled, dangerous JSON passes
	cfgDisabled := httpServer.Config{
		Addr:                     ":0",
		Mode:                     "test",
		DisableBlockSpecialChars: true,
	}
	sDisabled := httpServer.NewServer("test-app", lc, l, cfgDisabled, nil, nil)
	sDisabled.POST("/comment", func(c *gin.Context) {
		c.String(http.StatusOK, "saved")
	})

	reqBypass, _ := http.NewRequest(http.MethodPost, "/comment", bytes.NewBufferString(`{"text": "<script>alert(1)</script>"}`))
	reqBypass.Header.Set("Content-Type", "application/json")
	wBypass := httptest.NewRecorder()
	sDisabled.ServeHTTP(wBypass, reqBypass)
	assert.Equal(t, http.StatusOK, wBypass.Code)
}

func TestServer_SecurityHeaders(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr: ":0",
		Mode: "test",
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	s.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=16070400")
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "includeSubDomains")
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "preload")
	assert.Equal(t, "frame-ancestors 'none'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "no-store, no-cache, must-revalidate", w.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
}

func TestServer_Idempotency(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr: ":0",
		Mode: "test",
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	storage := httpServer.NewMemoryIdempotencyStorage()

	callCount := 0

	// Consumer route with custom canonical key based on reference ID in body
	s.POST("/checkout",
		httpServer.Idempotency(storage,
			httpServer.WithCanonicalKeyFunc(func(c *gin.Context, body []byte) (string, error) {
				var req struct {
					Ref string `json:"ref"`
				}
				if err := json.Unmarshal(body, &req); err == nil && req.Ref != "" {
					return "order:" + req.Ref, nil
				}
				return "", fmt.Errorf("ref is required")
			}),
		),
		func(c *gin.Context) {
			callCount++
			c.JSON(http.StatusOK, gin.H{"processed": true, "count": callCount})
		},
	)

	body := `{"ref": "REF-001", "total": 25000}`

	// 1. Initial request -> Handler executes
	req1, _ := http.NewRequest(http.MethodPost, "/checkout", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	s.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, 1, callCount)

	// 2. Duplicate retry with same ref -> Cached response replayed without executing handler
	req2, _ := http.NewRequest(http.MethodPost, "/checkout", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	s.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, "true", w2.Header().Get("X-Idempotent-Replayed"))
	assert.Equal(t, w1.Body.String(), w2.Body.String())
}

func TestServer_MiddlewarePipelineOrder_RecoveryAndTracing(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr:            ":0",
		Mode:            "test",
		ShowErrorDetail: true,
	}

	s := httpServer.NewServer("order-test-app", lc, l, cfg, nil, nil)

	s.GET("/panic-route", func(c *gin.Context) {
		panic("unexpected crash in handler")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic-route", nil)
	s.ServeHTTP(w, req)

	// Step 2 (Recovery + Error) should catch the panic and return 500 JSON envelope
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp struct {
		Source  string `json:"source"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "order-test-app", resp.Source)
	assert.Contains(t, resp.Error, "unexpected crash in handler")

	// Step 3 (Tracer) should have injected X-Trace-Id and X-Session-Id
	assert.NotEmpty(t, w.Header().Get("x-trace-id"))
	assert.NotEmpty(t, w.Header().Get("x-session-id"))

	// Step 7 (Security Headers) should also be present
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestServer_MetadataExtraction(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr: ":0",
		Mode: "test",
		Metadata: coremetadata.Config{
			CaptureHeaders: []string{"X-Tenant-Id"},
		},
	}

	s := httpServer.NewServer("meta-test-app", lc, l, cfg, nil, nil)

	var extractedMeta *coremetadata.Metadata
	s.GET("/test-meta", func(c *gin.Context) {
		extractedMeta, _ = coremetadata.FromContext(c.Request.Context())
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest(http.MethodGet, "/test-meta", nil)
	req.Header.Set("X-Trace-Id", "trace-777")
	req.Header.Set("X-Tenant-Id", "tenant-pegadaian")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, extractedMeta)
	assert.Equal(t, "trace-777", extractedMeta.TraceID)
	assert.Equal(t, "tenant-pegadaian", extractedMeta.Get("x-tenant-id"))
}

type testMockSecurityManager struct {
	authFunc      func(ctx context.Context, cred securityCore.Credential) (*securityCore.SecurityContext, error)
	authorizeFunc func(ctx context.Context, permission string) error
}

func (m *testMockSecurityManager) Authenticate(ctx context.Context, cred securityCore.Credential) (*securityCore.SecurityContext, error) {
	if m.authFunc != nil {
		return m.authFunc(ctx, cred)
	}
	return &securityCore.SecurityContext{
		Authenticated: true,
		Principal:     &securityCore.Principal{Subject: "user-123"},
	}, nil
}

func (m *testMockSecurityManager) AuthenticateSession(ctx context.Context, sessionID string) (*securityCore.SecurityContext, error) {
	return &securityCore.SecurityContext{
		Authenticated: true,
		Principal:     &securityCore.Principal{Subject: "user-session"},
	}, nil
}

func (m *testMockSecurityManager) Authorize(ctx context.Context, permission string) error {
	if m.authorizeFunc != nil {
		return m.authorizeFunc(ctx, permission)
	}
	return nil
}

func TestServer_SecurityMiddleware_DeclarativeConfig(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr:            ":0",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Mode:            "test",
		Security: &httpSecurity.Config{
			Required:     true,
			EnableBearer: true,
		},
	}

	s := httpServer.NewServer("sec-test-app", lc, l, cfg, nil, nil)
	mgr := &testMockSecurityManager{}
	s.SetSecurityManager(mgr)

	var authenticatedSubject string
	s.GET("/protected", s.SecurityMiddleware(), func(c *gin.Context) {
		secCtx, ok := securityCore.FromContext(c.Request.Context())
		if ok && secCtx != nil && secCtx.Principal != nil {
			authenticatedSubject = secCtx.Principal.Subject
		}
		c.String(http.StatusOK, "welcome")
	})

	// 1. Missing auth header -> 401 Unauthorized
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 2. Valid Bearer token -> 200 OK & subject injected
	req, _ = http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token-abc")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-123", authenticatedSubject)
}

func TestServer_SecurityMiddleware_RequirePermission(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr:            ":0",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Mode:            "test",
	}

	s := httpServer.NewServer("perm-test-app", lc, l, cfg, nil, nil)
	allowed := true
	mgr := &testMockSecurityManager{
		authorizeFunc: func(ctx context.Context, permission string) error {
			if !allowed {
				return securityCore.ErrForbidden
			}
			return nil
		},
	}
	s.SetSecurityManager(mgr, httpSecurity.Config{
		Required:     true,
		EnableBearer: true,
	})

	s.GET("/admin", s.SecurityMiddleware(), s.RequirePermission("admin:access"), func(c *gin.Context) {
		c.String(http.StatusOK, "admin-dashboard")
	})

	// Allowed permission
	req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "admin-dashboard", w.Body.String())

	// Forbidden permission -> 403
	allowed = false
	req, _ = http.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestServer_PackageLevelSecurityHelpers(t *testing.T) {
	mgr := &testMockSecurityManager{}

	secMw := httpServer.Security(mgr, httpSecurity.Config{
		Required:     true,
		EnableBearer: true,
	})
	permMw := httpServer.RequirePermission(mgr, "read:users")

	r := gin.New()
	r.GET("/api/users", secMw, permMw, func(c *gin.Context) {
		c.String(http.StatusOK, "users-list")
	})

	req, _ := http.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "users-list", w.Body.String())
}
