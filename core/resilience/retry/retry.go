package retry

import (
	"context"
	"time"
)

// Policy defines the retry configuration.
type Policy struct {
	Enabled     bool          `mapstructure:"enabled"`
	MaxAttempts int           `mapstructure:"max_attempts" validate:"required,gte=1"`
	Delay       time.Duration `mapstructure:"delay" validate:"required"`
}

// New defines a new retry policy.
func New(cfg Policy) *Policy {
	return &Policy{
		MaxAttempts: cfg.MaxAttempts,
		Delay:       cfg.Delay,
	}
}

// Execute executes a function with a retry mechanism.
func (p *Policy) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	var err error
	for i := 0; i < p.MaxAttempts; i++ {
		err = fn(ctx)
		if err == nil {
			return nil
		}

		if i < p.MaxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.Delay):
			}
		}
	}
	return err
}
