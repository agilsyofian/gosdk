package health_test

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/stretchr/testify/assert"
)

func TestHealthRegistry_Defaults(t *testing.T) {
	ctx := context.Background()
	reg := health.NewRegistry()

	// Default when no checks are registered
	liveness := reg.CheckLiveness(ctx)
	assert.Equal(t, health.StatusUp, liveness.Status)

	readiness := reg.CheckReadiness(ctx)
	assert.Equal(t, health.StatusUp, readiness.Status)

	overall := reg.Check(ctx)
	assert.Equal(t, health.StatusUp, overall.Status)
}

func TestHealthRegistry_LivenessAndReadinessSeparation(t *testing.T) {
	ctx := context.Background()
	reg := health.NewRegistry()

	// Register liveness check as UP
	reg.RegisterLivenessFunc("process", func(ctx context.Context) health.Health {
		return health.Up()
	})

	// Register readiness check as DOWN (e.g. database down)
	reg.RegisterReadinessFunc("database", func(ctx context.Context) health.Health {
		return health.DownWithDetails(map[string]any{"reason": "connection refused"})
	})

	// Liveness must remain UP (container should NOT be restarted by K8s)
	liveness := reg.CheckLiveness(ctx)
	assert.Equal(t, health.StatusUp, liveness.Status)
	assert.Contains(t, liveness.Details, "process")
	assert.NotContains(t, liveness.Details, "database")

	// Readiness must be DOWN (traffic should NOT be routed to this pod)
	readiness := reg.CheckReadiness(ctx)
	assert.Equal(t, health.StatusDown, readiness.Status)
	assert.Contains(t, readiness.Details, "database")
	assert.NotContains(t, readiness.Details, "process")

	// Overall check aggregates both
	overall := reg.Check(ctx)
	assert.Equal(t, health.StatusDown, overall.Status)
	assert.Contains(t, overall.Details, "liveness:process")
	assert.Contains(t, overall.Details, "readiness:database")
}

func TestHealthRegistry_PanicRecovery(t *testing.T) {
	ctx := context.Background()
	reg := health.NewRegistry()

	reg.RegisterReadinessFunc("panicky", func(ctx context.Context) health.Health {
		var nilPtr *string
		_ = *nilPtr // force panic
		return health.Up()
	})

	readiness := reg.CheckReadiness(ctx)
	assert.Equal(t, health.StatusDown, readiness.Status)
	assert.Contains(t, readiness.Details, "panicky")
}

func TestHealthRegistry_BackwardCompatibility(t *testing.T) {
	ctx := context.Background()
	reg := health.NewRegistry()

	reg.Register("legacy_check", health.CheckFunc(func(ctx context.Context) health.Health {
		return health.Up()
	}))

	readiness := reg.CheckReadiness(ctx)
	assert.Equal(t, health.StatusUp, readiness.Status)
	assert.Contains(t, readiness.Details, "legacy_check")
}
