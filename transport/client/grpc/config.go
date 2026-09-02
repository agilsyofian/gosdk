package grpc

import (
	"time"

	"github.com/agilsyofian/gosdk/resilience/circuitbreaker"
	"github.com/agilsyofian/gosdk/resilience/retry"
)

type Config struct {
	ServerName     string                `mapstructure:"server_name" validate:"required"`
	Addr           string                `mapstructure:"addr" validate:"required"`
	Timeout        time.Duration         `mapstructure:"timeout" validate:"required"`
	Retry          retry.Policy          `mapstructure:"retry" validate:"required"`
	CircuitBreaker circuitbreaker.Config `mapstructure:"circuit_breaker" validate:"required"`
}
