package observability

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
)

// Config aggregates configuration for all observability components.
type Config struct {
	Logger  logger.Config  `mapstructure:"logger"`
	Tracing tracing.Config `mapstructure:"tracing"`
}

// Observability bundles initialized logger, tracing, and health components.
type Observability struct {
	Logger  logger.Logger
	Tracing *tracing.Tracing
	Health  health.Registry
}

// Init initializes the core observability stack (Logger, Tracing, Health registry).
func Init(cfg Config, appName, appVersion string) (*Observability, error) {
	log := logger.NewZerologLogger(cfg.Logger)

	tracer, err := tracing.NewTracing(cfg.Tracing, appName, appVersion)
	if err != nil {
		return nil, err
	}

	return &Observability{
		Logger:  log,
		Tracing: tracer,
		Health:  health.NewRegistry(),
	}, nil
}

// Shutdown gracefully flushes and shuts down tracing exporters.
func (o *Observability) Shutdown(ctx context.Context) error {
	if o == nil || o.Tracing == nil {
		return nil
	}
	return o.Tracing.Shutdown(ctx)
}
