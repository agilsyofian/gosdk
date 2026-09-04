package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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
	httpServer     *http.Server
	logger         logger.Logger
	engine         *gin.Engine
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

	if cfg.Mode != "" {
		gin.SetMode(cfg.Mode)
	}

	engine := gin.New()
	// Default middleware
	engine.Use(
		gin.Recovery(),
		otelgin.Middleware(appName),
		middleware.Metadata(),
		middleware.Tracer(appName),
		middleware.Logger(l, appName),
		middleware.Error(l, appName, cfg.ShowErrorDetail),
		middleware.Recovery(appName),
	)

	if cfg.EnableSwagger {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	if cfg.Actuator.Enabled {
		actuator := engine.Group(cfg.Actuator.Path)
		{
			actuator.GET(cfg.Actuator.HealthPath, func(c *gin.Context) {
				h := hr.Check(c.Request.Context())
				status := http.StatusOK
				if h.Status == health.StatusDown {
					status = http.StatusServiceUnavailable
				}
				c.JSON(status, h)
			})

			actuator.GET(cfg.Actuator.InfoPath, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"app":  appName,
					"mode": cfg.Mode,
				})
			})
		}
	}

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	s := &Server{
		httpServer:     httpServer,
		logger:         l,
		engine:         engine,
		healthRegistry: hr,
	}

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

	return s
}

func (s *Server) Router() *gin.Engine {
	return s.engine
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
