package health

import (
	"context"
	"sync"
)

type Status string

const (
	StatusUp   Status = "UP"
	StatusDown Status = "DOWN"
)

type Health struct {
	Status  Status         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
}

// Up returns a Health result with StatusUp.
func Up() Health {
	return Health{Status: StatusUp}
}

// Down returns a Health result with StatusDown.
func Down() Health {
	return Health{Status: StatusDown}
}

// DownWithDetails returns a Health result with StatusDown and details.
func DownWithDetails(details map[string]any) Health {
	return Health{
		Status:  StatusDown,
		Details: details,
	}
}

// Check evaluates the health of a component.
type Check interface {
	Check(ctx context.Context) Health
}

// CheckFunc is an adapter to allow the use of ordinary functions as health checks.
type CheckFunc func(ctx context.Context) Health

// Check calls f(ctx).
func (f CheckFunc) Check(ctx context.Context) Health {
	return f(ctx)
}

// Registry manages and aggregates liveness and readiness health checks.
// It is designed to be transport-agnostic (used by HTTP Actuator and gRPC Health service).
type Registry interface {
	// Register registers a check as a readiness check (preserving backward compatibility).
	Register(name string, check Check)

	// RegisterFunc registers an ordinary function as a readiness check.
	RegisterFunc(name string, fn func(ctx context.Context) Health)

	// RegisterLiveness registers a check for application liveness (Kubernetes liveness probe).
	// If this check fails, Kubernetes will restart the container.
	RegisterLiveness(name string, check Check)

	// RegisterLivenessFunc registers an ordinary function as a liveness check.
	RegisterLivenessFunc(name string, fn func(ctx context.Context) Health)

	// RegisterReadiness registers a check for application readiness (Kubernetes readiness probe).
	// If this check fails, Kubernetes will temporarily remove the pod from service endpoints.
	RegisterReadiness(name string, check Check)

	// RegisterReadinessFunc registers an ordinary function as a readiness check.
	RegisterReadinessFunc(name string, fn func(ctx context.Context) Health)

	// Check evaluates all registered checks (both liveness and readiness).
	Check(ctx context.Context) Health

	// CheckLiveness evaluates only registered liveness checks.
	// Returns StatusUp if no explicit liveness checks are registered.
	CheckLiveness(ctx context.Context) Health

	// CheckReadiness evaluates only registered readiness checks.
	// Returns StatusUp if no explicit readiness checks are registered.
	CheckReadiness(ctx context.Context) Health
}

type healthRegistry struct {
	livenessChecks  map[string]Check
	readinessChecks map[string]Check
	mu              sync.RWMutex
}

// NewRegistry constructs a new health check Registry.
func NewRegistry() Registry {
	return &healthRegistry{
		livenessChecks:  make(map[string]Check),
		readinessChecks: make(map[string]Check),
	}
}

func (r *healthRegistry) Register(name string, check Check) {
	r.RegisterReadiness(name, check)
}

func (r *healthRegistry) RegisterFunc(name string, fn func(ctx context.Context) Health) {
	r.RegisterReadiness(name, CheckFunc(fn))
}

func (r *healthRegistry) RegisterLiveness(name string, check Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.livenessChecks[name] = check
}

func (r *healthRegistry) RegisterLivenessFunc(name string, fn func(ctx context.Context) Health) {
	r.RegisterLiveness(name, CheckFunc(fn))
}

func (r *healthRegistry) RegisterReadiness(name string, check Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readinessChecks[name] = check
}

func (r *healthRegistry) RegisterReadinessFunc(name string, fn func(ctx context.Context) Health) {
	r.RegisterReadiness(name, CheckFunc(fn))
}

func (r *healthRegistry) Check(ctx context.Context) Health {
	r.mu.RLock()
	checks := make(map[string]Check, len(r.livenessChecks)+len(r.readinessChecks))
	for k, v := range r.livenessChecks {
		checks["liveness:"+k] = v
	}
	for k, v := range r.readinessChecks {
		checks["readiness:"+k] = v
	}
	r.mu.RUnlock()

	return executeChecks(ctx, checks)
}

func (r *healthRegistry) CheckLiveness(ctx context.Context) Health {
	r.mu.RLock()
	checks := make(map[string]Check, len(r.livenessChecks))
	for k, v := range r.livenessChecks {
		checks[k] = v
	}
	r.mu.RUnlock()

	return executeChecks(ctx, checks)
}

func (r *healthRegistry) CheckReadiness(ctx context.Context) Health {
	r.mu.RLock()
	checks := make(map[string]Check, len(r.readinessChecks))
	for k, v := range r.readinessChecks {
		checks[k] = v
	}
	r.mu.RUnlock()

	return executeChecks(ctx, checks)
}

func executeChecks(ctx context.Context, checks map[string]Check) Health {
	if len(checks) == 0 {
		return Health{
			Status: StatusUp,
		}
	}

	type checkResult struct {
		name   string
		health Health
	}

	results := make(chan checkResult, len(checks))
	var wg sync.WaitGroup

	for name, check := range checks {
		wg.Add(1)
		go func(n string, c Check) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					results <- checkResult{
						name: n,
						health: Health{
							Status: StatusDown,
							Details: map[string]any{
								"error": "panic recovered during health check execution",
							},
						},
					}
				}
			}()

			results <- checkResult{
				name:   n,
				health: c.Check(ctx),
			}
		}(name, check)
	}

	wg.Wait()
	close(results)

	overallStatus := StatusUp
	details := make(map[string]any, len(checks))

	for res := range results {
		if res.health.Status == StatusDown {
			overallStatus = StatusDown
		}
		details[res.name] = res.health
	}

	return Health{
		Status:  overallStatus,
		Details: details,
	}
}
