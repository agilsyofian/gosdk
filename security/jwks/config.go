package jwks

import "time"

type Config struct {

	// JWKSURL is the URI for the JWKS endpoint (RS256).
	JWKSURL string `mapstructure:"jwks_url" validate:"required,url"`

	// CacheTTL defines how long the JWKS keys are cached.
	// Defaults to 1 hour if not set.
	CacheTTL time.Duration `mapstructure:"cache_ttl" validate:"required,gt=0"`

	// RefreshTimeout defines the timeout for fetching JWKS.
	// Defaults to 5 seconds if not set.
	RefreshTimeout time.Duration `mapstructure:"refresh_timeout" validate:"required,gt=0"`
}
