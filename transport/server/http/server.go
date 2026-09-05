package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
	httpServer     *http.Server
	logger         logger.Logger
	engine         *gin.Engine
	baseRouter     gin.IRouter
	basePath       string
	healthRegistry health.Registry
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
	engine.Use(
		otelgin.Middleware(appName),
		middleware.Metadata(),
		middleware.Tracer(appName),
		middleware.Logger(l, appName, cfg.LoggerExcludeRoutes...),
		middleware.Error(l, appName, cfg.ShowErrorDetail),
		middleware.Recovery(appName),
	)

	basePath := normalizeBasePath(cfg.BasePath)
	var baseRouter gin.IRouter = engine
	if basePath != "" {
		baseRouter = engine.Group(basePath)
	}

	if cfg.EnableSwagger {
		setupSwagger(engine, baseRouter, basePath)
	}

	if cfg.Actuator.Enabled {
		setupActuator(baseRouter, cfg.Actuator, hr, appName, cfg.Mode)
	}

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	s := &Server{
		IRouter:        baseRouter,
		httpServer:     httpServer,
		logger:         l,
		engine:         engine,
		baseRouter:     baseRouter,
		basePath:       basePath,
		healthRegistry: hr,
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

func setupActuator(router gin.IRouter, cfg ActuatorConfig, hr health.Registry, appName, mode string) {
	actuator := router.Group(cfg.Path)

	healthPath := defaultIfEmpty(cfg.HealthPath, "/health")
	actuator.GET(healthPath, healthCheckHandler(hr.Check))

	livenessPath := defaultIfEmpty(cfg.LivenessPath, "/liveness")
	actuator.GET(livenessPath, healthCheckHandler(hr.CheckLiveness))

	readinessPath := defaultIfEmpty(cfg.ReadinessPath, "/readiness")
	actuator.GET(readinessPath, healthCheckHandler(hr.CheckReadiness))

	infoPath := defaultIfEmpty(cfg.InfoPath, "/info")
	actuator.GET(infoPath, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"app":  appName,
			"mode": mode,
		})
	})
}

func healthCheckHandler(checkFn func(context.Context) health.Health) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			go func() {
				if err := s.Start(); err != nil {
					l.Error(ctx, "http server failed to start", logger.LogPayload{})
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if t != nil {
				_ = t.Shutdown(ctx)
			}
			return s.Shutdown(ctx)
		},
	})
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

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info(ctx, "stopping http server", logger.LogPayload{
		Type:   "HTTP_SERVER",
		Method: "SHUTDOWN",
	})
	return s.httpServer.Shutdown(ctx)
}
