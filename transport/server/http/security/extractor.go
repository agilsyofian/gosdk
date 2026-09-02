// Package security adapts the transport-independent security core
// to Gin. It is responsible for extracting credentials from the HTTP
// request, calling the SecurityManager, injecting the resulting
// SecurityContext into the request context, and mapping security
// errors to HTTP responses.
//
// This package MUST NOT implement any authentication logic itself
// (JWT validation, password checks, etc) -- all of that lives in the
// security core and its authenticator subpackage.
package security

import (
	"strings"

	"github.com/agilsyofian/gosdk/security"
	"github.com/gin-gonic/gin"
)

// Config configures the Gin security middleware.
type Config struct {
	// Required, if true, causes the middleware to reject requests
	// with no valid credential (401). If false, unauthenticated
	// requests are allowed through with no SecurityContext in the
	// request context -- handlers should check core.FromContext
	// themselves when Required is false.
	Required bool

	// SessionCookie is the name of the cookie holding a session ID,
	// e.g. "SESSION_ID". Leave empty to disable cookie-based session
	// extraction.
	SessionCookie string

	// EnableBearer enables extraction of "Authorization: Bearer
	// <token>".
	EnableBearer bool

	// EnableBasic enables extraction of "Authorization: Basic
	// <base64>".
	EnableBasic bool

	// EnableAPIKey enables extraction of the API key header (see
	// APIKeyHeader).
	EnableAPIKey bool

	// APIKeyHeader is the header name used for API key extraction.
	// Defaults to "X-API-Key" if empty.
	APIKeyHeader string

	// AllowedTypes, if non-empty, restricts which CredentialTypes this
	// middleware instance will accept, regardless of which Enable*
	// flags are set. Useful for route groups that should only accept
	// e.g. session or API key auth.
	AllowedTypes []security.CredentialType
}

func (c Config) apiKeyHeader() string {
	if c.APIKeyHeader != "" {
		return c.APIKeyHeader
	}
	return "X-API-Key"
}

func (c Config) allows(t security.CredentialType) bool {
	if len(c.AllowedTypes) == 0 {
		return true
	}
	for _, allowed := range c.AllowedTypes {
		if allowed == t {
			return true
		}
	}
	return false
}

// extracted holds a candidate credential/session prior to conflict
// resolution.
type extracted struct {
	sessionID  string
	credential *security.Credential
}

// extractCredential extracts a credential (or session ID) from the
// request according to the configured, explicit precedence:
//
//  1. Session cookie, if configured
//  2. Authorization header (Bearer or Basic)
//  3. API Key header
//
// It fails closed on conflicting credentials (e.g. both a session
// cookie and a Bearer token present) rather than silently choosing a
// winner.
func extractCredential(c *gin.Context, cfg Config) (sessionID string, credential *security.Credential, err error) {
	var candidates []extracted

	if cfg.SessionCookie != "" {
		if v, cookieErr := c.Cookie(cfg.SessionCookie); cookieErr == nil && v != "" {
			candidates = append(candidates, extracted{sessionID: v})
		}
	}

	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		switch {
		case cfg.EnableBearer && cfg.allows(security.CredentialBearer) && hasPrefixFold(authHeader, "Bearer "):
			token := strings.TrimSpace(authHeader[len("Bearer "):])
			candidates = append(candidates, extracted{credential: &security.Credential{
				Type:  security.CredentialBearer,
				Value: token,
			}})
		case cfg.EnableBasic && cfg.allows(security.CredentialBasic) && hasPrefixFold(authHeader, "Basic "):
			payload := strings.TrimSpace(authHeader[len("Basic "):])
			candidates = append(candidates, extracted{credential: &security.Credential{
				Type:  security.CredentialBasic,
				Value: payload,
			}})
		}
	}

	if cfg.EnableAPIKey && cfg.allows(security.CredentialAPIKey) {
		if key := c.GetHeader(cfg.apiKeyHeader()); key != "" {
			candidates = append(candidates, extracted{credential: &security.Credential{
				Type:  security.CredentialAPIKey,
				Value: key,
			}})
		}
	}

	switch len(candidates) {
	case 0:
		return "", nil, security.ErrNoCredential
	case 1:
		return candidates[0].sessionID, candidates[0].credential, nil
	default:
		return "", nil, security.ErrConflictingCredential
	}
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}
