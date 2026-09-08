// Package jwt implements core.Authenticator for Bearer JWTs
// (HS256/RS256), as an independent Go module.
//
// This package MUST NOT depend on Gin, net/http, or gRPC -- only on
// the security core module and the standard library.
package jwt

import (
	"context"
	"fmt"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/security"
	jwksManager "github.com/agilsyofian/gosdk/security/jwks"
)

// Authenticator validates Bearer JWTs.
//
// It validates the signature first, then exp/nbf/iss/aud as
// configured, before ever trusting a claim from the token -- signature
// validation always precedes session resolution.
//
// A JWT should preferably carry only authentication claims (sub, sid,
// iat, exp). When the token carries a "sid" claim, this authenticator
// returns it as AuthenticationResult.SessionID and leaves Principal
// nil, deferring to the SecurityManager to resolve the authoritative
// Principal from the server-side session. When no "sid" is present,
// this authenticator falls back to building a Principal directly from
// the token's claims for stateless use cases; this is not the
// recommended default for the primary FE -> BE flow.
type Authenticator struct {
	cfg  Config
	jwks jwksManager.Manager
}

// New constructs a JWT Authenticator.
func New(cfg Config, l logger.Logger) (security.Authenticator, error) {
	if cfg.JWKSURL == "" {
		panic("jwt: JWKSURL is not configured")
	}

	cfgJwks := jwksManager.Config{
		JWKSURL:        cfg.JWKSURL,
		CacheTTL:       cfg.CacheTTL,
		RefreshTimeout: cfg.RefreshTimeout,
	}

	return &Authenticator{
		cfg:  cfg,
		jwks: jwksManager.NewJWKSManager(cfgJwks, l),
	}, nil
}

var _ security.Authenticator = (*Authenticator)(nil)

// Type implements core.Authenticator.
func (a *Authenticator) Type() security.CredentialType {
	return security.CredentialBearer
}

// Authenticate implements core.Authenticator.
func (a *Authenticator) Authenticate(ctx context.Context, credential security.Credential) (*security.AuthenticationResult, error) {
	if credential.Type != security.CredentialBearer {
		return nil, fmt.Errorf("%w: %s", security.ErrUnsupportedCredential, credential.Type)
	}
	if credential.Value == "" {
		return nil, fmt.Errorf("%w", security.ErrNoCredential)
	}

	claims, err := parseAndVerify(ctx, credential.Value, verifyConfig{
		JWKS:     a.jwks,
		Issuer:   a.cfg.Issuer,
		Audience: a.cfg.Audience,
		Leeway:   a.cfg.Leeway,
	})
	if err != nil {
		// Do not leak internal validation details (see security rules
		// in security/errors.go) -- collapse to the sentinel error.
		return nil, fmt.Errorf("%w", security.ErrInvalidCredential)
	}

	if claims.Issuer != a.cfg.Issuer {
		return nil, security.ErrInvalidCredential
	}

	if claims.SessionID != "" {
		// Stateful path (recommended default): defer to the
		// SecurityManager/SessionManager to resolve the authoritative
		// Principal.
		return &security.AuthenticationResult{
			Authenticated: true,
			AuthType:      security.CredentialBearer,
			SessionID:     claims.SessionID,
		}, nil
	}

	// Stateless fallback: build a Principal straight from the token.
	principal := &security.Principal{
		Subject: claims.Subject,
		Claims:  claims.Extra,
	}

	return &security.AuthenticationResult{
		Authenticated: true,
		AuthType:      security.CredentialBearer,
		Principal:     principal,
	}, nil
}
