package timeout

import (
	"context"
	"time"
)

// Policy defines the timeout configuration.
type Policy struct {
	Timeout time.Duration
}

// New defines a new timeout policy.
func New(timeout time.Duration) *Policy {
	return &Policy{Timeout: timeout}
}

// Execute executes a function with a timeout.
func (p *Policy) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if p.Timeout <= 0 {
		return fn(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	return fn(ctx)
}
