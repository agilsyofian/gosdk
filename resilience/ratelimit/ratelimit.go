package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// Policy defines the rate limit configuration.
type Policy struct {
	limiter *rate.Limiter
}

// New defines a new rate limiter.
// r: limit per second.
// b: burst size.
func New(r float64, b int) *Policy {
	return &Policy{
		limiter: rate.NewLimiter(rate.Limit(r), b),
	}
}

// Execute executes a function with rate limit constraints.
func (p *Policy) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := p.limiter.Wait(ctx); err != nil {
		return err
	}
	return fn(ctx)
}
