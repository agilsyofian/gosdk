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

type Check interface {
	Check(ctx context.Context) Health
}

type Registry interface {
	Register(name string, check Check)
	Check(ctx context.Context) Health
}

type healthRegistry struct {
	checks map[string]Check
	mu     sync.RWMutex
}

func NewRegistry() Registry {
	return &healthRegistry{
		checks: make(map[string]Check),
	}
}

func (r *healthRegistry) Register(name string, check Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = check
}

func (r *healthRegistry) Check(ctx context.Context) Health {
	r.mu.RLock()
	defer r.mu.RUnlock()

	overallStatus := StatusUp
	details := make(map[string]any)

	for name, check := range r.checks {
		h := check.Check(ctx)
		if h.Status == StatusDown {
			overallStatus = StatusDown
		}
		details[name] = h
	}

	return Health{
		Status:  overallStatus,
		Details: details,
	}
}
