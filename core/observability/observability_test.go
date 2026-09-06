package observability_test

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
)

func TestObservability_InitAndShutdown(t *testing.T) {
	cfg := observability.Config{
		Logger: logger.DefaultConfig(),
		Tracing: tracing.Config{
			Enabled: false,
		},
	}

	obs, err := observability.Init(cfg, "test-app", "v1.0.0")
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if obs == nil {
		t.Fatal("Init() returned nil Observability instance")
	}
	if obs.Logger == nil {
		t.Error("expected non-nil Logger")
	}
	if obs.Tracing == nil {
		t.Error("expected non-nil Tracing")
	}
	if obs.Health == nil {
		t.Error("expected non-nil Health registry")
	}

	ctx := context.Background()
	if err := obs.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
