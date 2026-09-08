// Package security adapts the transport-independent security core
// to gRPC via unary/stream server interceptors. It MUST use the same
// SecurityManager as any other transport adapter (e.g. Gin) -- never
// a second, parallel security implementation.
package security

import (
	"strings"

	"github.com/agilsyofian/gosdk/security"
	"google.golang.org/grpc/metadata"
)

// Config configures the gRPC security interceptors. It mirrors
// gincore.Config so the same authentication conventions apply
// across transports.
type Config struct {
	// Required, if true, causes the interceptor to reject calls with
	// no valid credential (codes.Unauthenticated).
	Required bool `mapstructure:"required"`

	// SessionMetadataKey is the metadata key holding a session ID,
	// e.g. "session-id". Leave empty to disable.
	SessionMetadataKey string `mapstructure:"session_metadata_key"`

	EnableBearer bool `mapstructure:"enable_bearer"`
	EnableBasic  bool `mapstructure:"enable_basic"`
	EnableAPIKey bool `mapstructure:"enable_api_key"`

	// APIKeyMetadataKey is the metadata key used for API key
	// extraction. Defaults to "x-api-key" if empty.
	APIKeyMetadataKey string `mapstructure:"api_key_metadata_key"`

	AllowedTypes []security.CredentialType `mapstructure:"allowed_types"`
}

func (c Config) apiKeyMetadataKey() string {
	if c.APIKeyMetadataKey != "" {
		return c.APIKeyMetadataKey
	}
	return "x-api-key"
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

type extracted struct {
	sessionID  string
	credential *security.Credential
}

// extractCredential extracts a credential (or session ID) from gRPC
// metadata following the SDK's defined conventions:
//
//	authorization  -> Bearer JWT / Basic
//	x-api-key      -> API Key
//	session-id     -> Session
//
// Extraction precedence mirrors the Gin adapter: session metadata
// first, then the authorization header, then the API key header. It
// fails closed on conflicting credentials.
func extractCredential(md metadata.MD, cfg Config) (sessionID string, credential *security.Credential, err error) {
	var candidates []extracted

	if cfg.SessionMetadataKey != "" {
		if v := firstValue(md, cfg.SessionMetadataKey); v != "" {
			candidates = append(candidates, extracted{sessionID: v})
		}
	}

	if authHeader := firstValue(md, "authorization"); authHeader != "" {
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
		if key := firstValue(md, cfg.apiKeyMetadataKey()); key != "" {
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

func firstValue(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}
