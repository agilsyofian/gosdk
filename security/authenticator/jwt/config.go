package jwt

import (
	"time"
)

// Config configures an Authenticator.
type Config struct {
	// Issuer, if set, is validated against the token's "iss" claim.
	Issuer string `mapstructure:"issuer" validate:"required"`

	// Audience, if set, is validated against the token's "aud" claim.
	Audience string `mapstructure:"audience" validate:"required"`

	// JWKSURL is the URI for the JWKS endpoint (RS256).
	JWKSURL string `mapstructure:"jwks_url" validate:"required,url"`

	// CacheTTL defines how long the JWKS keys are cached.
	// Defaults to 1 hour if not set.
	CacheTTL time.Duration `mapstructure:"cache_ttl" validate:"required,gt=0"`

	// RefreshTimeout defines the timeout for fetching JWKS.
	// Defaults to 5 seconds if not set.
	RefreshTimeout time.Duration `mapstructure:"refresh_timeout" validate:"required,gt=0"`

	// Kid is the Key ID to be placed in the JWT header (issuance only).
	Kid string `mapstructure:"kid"`

	// Leeway allows small clock skew when validating exp/nbf.
	// Defaults to zero.
	Leeway time.Duration `mapstructure:"leeway"`
}
