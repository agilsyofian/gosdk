package http

import (
	"time"

	"github.com/agilsyofian/gosdk/core/resilience/circuitbreaker"
	"github.com/agilsyofian/gosdk/core/resilience/retry"
)

type Config struct {
	ServerName     string                `mapstructure:"server_name" validate:"required"`
	BaseURL        string                `mapstructure:"base_url" validate:"required"`
	Timeout        time.Duration         `mapstructure:"timeout" validate:"required"`
	Retry          retry.Policy          `mapstructure:"retry" validate:"required"`
	CircuitBreaker circuitbreaker.Config `mapstructure:"circuit_breaker" validate:"required"`
}
