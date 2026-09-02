package circuitbreaker

import (
	"context"
	"time"

	"github.com/sony/gobreaker/v2"
)

type Policy struct {
	cb *gobreaker.CircuitBreaker[any]
}

type Config struct {
	Enabled     bool          `mapstructure:"enabled"`
	Name        string        `mapstructure:"name" validate:"required"`
	MinRequests uint32        `mapstructure:"min_requests" validate:"required"`
	MaxRequests uint32        `mapstructure:"max_requests" validate:"required"`
	Interval    time.Duration `mapstructure:"interval" validate:"required"`
	Timeout     time.Duration `mapstructure:"timeout" validate:"required"`
	FailureRate float64       `mapstructure:"failure_rate" validate:"required"`
}

func New(cfg Config) *Policy {
	st := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(c gobreaker.Counts) bool {
			if c.Requests < cfg.MinRequests {
				return false
			}
			rate := float64(c.TotalFailures) / float64(c.Requests)
			return rate >= cfg.FailureRate
		},
	}

	return &Policy{
		cb: gobreaker.NewCircuitBreaker[any](st),
	}
}

func (p *Policy) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	_, err := p.cb.Execute(func() (any, error) {
		err := fn(ctx)
		return nil, err
	})
	return err
}
