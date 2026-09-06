package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.uber.org/fx"
)

type Server struct {
	gin.IRouter
	httpServer         *http.Server
	logger             logger.Logger
	engine             *gin.Engine
	baseRouter         gin.IRouter
	basePath           string
	healthRegistry     health.Registry
	shutdownTimeout    time.Duration
	shutdownDelay      time.Duration
	inGracefulShutdown atomic.Bool
	cancelOngoing      context.CancelFunc
	stopSigWatcher     chan struct{}
}

func NewServer(
	appName string,
	lc fx.Lifecycle,
	l logger.Logger,
	cfg Config,
	t *tracing.Tracing,
	hr health.Registry,
) *Server {
	if hr == nil {
		hr = health.NewRegistry()
	}

	if cfg.Mode != "" {
		gin.SetMode(cfg.Mode)
	}

	engine := gin.New()

	// 1. CORS Middleware (Handle OPTIONS preflight & inject Vary: Origin)
	if cfg.EnableCors {
		engine.Use(middleware.CORS(middleware.CorsConfig{
			AllowOrigins:     cfg.Cors.AllowOrigins,
			AllowMethods:     cfg.Cors.AllowMethods,
			AllowHeaders:     cfg.Cors.AllowHeaders,
			ExposeHeaders:    cfg.Cors.ExposeHeaders,
			AllowCredentials: cfg.Cors.AllowCredentials,
			MaxAge:           cfg.Cors.MaxAge,
		}))
	}

	// 2. Recovery Middleware (Catch panics & format standardized error response)
	engine.Use(
		middleware.Error(l, appName, cfg.ShowErrorDetail),
		middleware.Recovery(appName),
	)

	// 3. Tracer Middleware (Extract OpenTelemetry trace context)
	engine.Use(
		otelgin.Middleware(appName),
		middleware.Tracer(appName),
	)

	// 4. Metadata Middleware (Extract Client IP, Device, Session, & Request IDs)
	engine.Use(middleware.Metadata())

	// 5. Block Method Override (Block X-HTTP-Method-Override & query _method)
	if !cfg.DisableBlockMethodOverride {
		engine.Use(middleware.BlockMethodOverride(appName))
	}

	// 6. Block Special Characters (Scan for XSS / SQLi in JSON string payloads)
	if !cfg.DisableBlockSpecialChars {
		engine.Use(middleware.BlockJSONSpecialChars(appName, cfg.AllowedSpecialCharsRegex))
	}

	// 7. Security Headers (Inject HSTS, CSP, X-Frame-Options, Nosniff, Cache-Control)
	engine.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
		Disabled:              cfg.SecurityHeaders.Disabled,
		HSTSMaxAge:            cfg.SecurityHeaders.HSTSMaxAge,
		DisableHSTSSubdomains: cfg.SecurityHeaders.DisableHSTSSubdomains,
		DisableHSTSPreload:    cfg.SecurityHeaders.DisableHSTSPreload,
		ContentSecurityPolicy: cfg.SecurityHeaders.ContentSecurityPolicy,
		FrameOptions:          cfg.SecurityHeaders.FrameOptions,
		ReferrerPolicy:        cfg.SecurityHeaders.ReferrerPolicy,
		DisableNoCache:        cfg.SecurityHeaders.DisableNoCache,
		SanitizeResponse:      cfg.SecurityHeaders.SanitizeResponse,
	}))

	// 8. Logger Middleware (Structured logging with dynamic route exclusions)
	engine.Use(middleware.Logger(l, appName, cfg.LoggerExcludeRoutes...))

	basePath := normalizeBasePath(cfg.BasePath)
	var baseRouter gin.IRouter = engine
	if basePath != "" {
		baseRouter = engine.Group(basePath)
	}

	if cfg.EnableSwagger {
		setupSwagger(engine, baseRouter, basePath)
	}

	var s *Server

	if cfg.Actuator.Enabled {
		setupActuator(baseRouter, cfg.Actuator, hr, appName, cfg.Mode, func() bool {
			return s != nil && s.inGracefulShutdown.Load()
		})
	}

	ongoingCtx, cancelOngoing := context.WithCancel(context.Background())

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		BaseContext: func(_ net.Listener) context.Context {
			return ongoingCtx
		},
	}

	s = &Server{
		IRouter:         baseRouter,
		httpServer:      httpServer,
		logger:          l,
		engine:          engine,
		baseRouter:      baseRouter,
		basePath:        basePath,
		healthRegistry:  hr,
		shutdownTimeout: cfg.ShutdownTimeout,
		shutdownDelay:   cfg.ShutdownDelay,
		cancelOngoing:   cancelOngoing,
	}

	registerLifecycle(lc, s, t, l)

	return s
}

func normalizeBasePath(raw string) string {
	basePath := strings.TrimSpace(raw)
	if basePath == "" {
		return ""
	}
	basePath = "/" + strings.Trim(basePath, "/")
	if basePath == "/" {
		return ""
	}
	return basePath
}

func setupSwagger(engine *gin.Engine, baseRouter gin.IRouter, basePath string) {
	swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler)
	if basePath != "" {
		baseRouter.GET("/swagger/*any", swaggerHandler)
		return
	}
	engine.GET("/swagger/*any", swaggerHandler)
}

func setupActuator(router gin.IRouter, cfg ActuatorConfig, hr health.Registry, appName, mode string, isShuttingDown func() bool) {
	actuator := router.Group(cfg.Path)

	healthPath := defaultIfEmpty(cfg.HealthPath, "/health")
	actuator.GET(healthPath, healthCheckHandler(hr.Check, isShuttingDown))

	livenessPath := defaultIfEmpty(cfg.LivenessPath, "/liveness")
	actuator.GET(livenessPath, healthCheckHandler(hr.CheckLiveness, isShuttingDown))

	readinessPath := defaultIfEmpty(cfg.ReadinessPath, "/readiness")
	actuator.GET(readinessPath, healthCheckHandler(hr.CheckReadiness, isShuttingDown))

	infoPath := defaultIfEmpty(cfg.InfoPath, "/info")
	actuator.GET(infoPath, func(c *gin.Context) {
		if isShuttingDown != nil && isShuttingDown() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "DOWN",
				"message": "server is shutting down",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"app":  appName,
			"mode": mode,
		})
	})
}

func healthCheckHandler(checkFn func(context.Context) health.Health, isShuttingDown func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isShuttingDown != nil && isShuttingDown() {
			c.JSON(http.StatusServiceUnavailable, health.Health{
				Status: health.StatusDown,
				Details: map[string]any{
					"message": "server is shutting down",
				},
			})
			return
		}

		h := checkFn(c.Request.Context())
		status := http.StatusOK
		if h.Status == health.StatusDown {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, h)
	}
}

func defaultIfEmpty(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func registerLifecycle(lc fx.Lifecycle, s *Server, t *tracing.Tracing, l logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			s.startSignalWatcher()
			go func() {
				if err := s.Start(); err != nil {
					l.Error(ctx, "http server failed to start", logger.LogPayload{})
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.stopSignalWatcherOnce()
			if t != nil {
				_ = t.Shutdown(ctx)
			}
			return s.Shutdown(ctx)
		},
	})
}

func (s *Server) startSignalWatcher() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	s.stopSigWatcher = make(chan struct{})

	go func() {
		select {
		case <-sigChan:
			s.inGracefulShutdown.Store(true)
			s.logger.Warn(context.Background(), "shutdown signal received, health probes will now return 503", logger.LogPayload{
				Type:   "HTTP_SERVER",
				Method: "SHUTDOWN",
			})
		case <-s.stopSigWatcher:
			signal.Stop(sigChan)
			return
		}
	}()
}

func (s *Server) stopSignalWatcherOnce() {
	if s.stopSigWatcher != nil {
		select {
		case <-s.stopSigWatcher:
		default:
			close(s.stopSigWatcher)
		}
	}
}

// Router returns the base router (gin.IRouter) under the configured BasePath.
// If BasePath is empty, it returns the root *gin.Engine.
// Any routes registered on Router() will automatically be prefixed with BasePath.
func (s *Server) Router() gin.IRouter {
	return s.baseRouter
}

// BaseRouter returns the router under the configured BasePath (alias for Router()).
func (s *Server) BaseRouter() gin.IRouter {
	return s.baseRouter
}

// Engine returns the underlying root *gin.Engine.
// Use this if you need direct access to the root engine (e.g. for engine-level configuration or ServeHTTP in tests).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// ServeHTTP implements http.Handler, delegating to the underlying gin engine.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.engine.ServeHTTP(w, req)
}

// BasePath returns the normalized base path (e.g. "/api/v1" or "").
func (s *Server) BasePath() string {
	return s.basePath
}

// Group creates a new router group relative to the BaseRouter.
// For example, if BasePath is "/api/v1", s.Group("/users") registers routes under "/api/v1/users".
func (s *Server) Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup {
	return s.baseRouter.Group(relativePath, handlers...)
}

func (s *Server) HealthRegistry() health.Registry {
	return s.healthRegistry
}

func (s *Server) Start() error {
	s.logger.Info(context.Background(), "starting http server", logger.LogPayload{
		Type:      "HTTP_SERVER",
		Method:    "START",
		IpAddress: fmt.Sprintf("%v", s.httpServer.Addr),
	})
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// InGracefulShutdown reports whether the server has entered the graceful shutdown phase.
func (s *Server) InGracefulShutdown() bool {
	return s.inGracefulShutdown.Load()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.inGracefulShutdown.Store(true)
	s.stopSignalWatcherOnce()

	s.logger.Info(ctx, "stopping http server", logger.LogPayload{
		Type:   "HTTP_SERVER",
		Method: "SHUTDOWN",
	})

	s.drainShutdownDelay(ctx)

	timeout := s.shutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go s.watchShutdownTimeout(shutdownCtx)

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error(ctx, fmt.Sprintf("http server graceful shutdown failed: %v", err), logger.LogPayload{
			Type:   "HTTP_SERVER",
			Method: "SHUTDOWN",
		})
		_ = s.httpServer.Close()
		return err
	}
	return nil
}

func (s *Server) drainShutdownDelay(ctx context.Context) {
	if s.shutdownDelay <= 0 {
		return
	}
	s.logger.Info(ctx, fmt.Sprintf("waiting %v drain delay before closing listener", s.shutdownDelay), logger.LogPayload{
		Type:   "HTTP_SERVER",
		Method: "SHUTDOWN",
	})
	select {
	case <-time.After(s.shutdownDelay):
	case <-ctx.Done():
	}
}

func (s *Server) watchShutdownTimeout(ctx context.Context) {
	<-ctx.Done()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) && s.cancelOngoing != nil {
		s.cancelOngoing()
	}
}
