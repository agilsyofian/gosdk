package jwt

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

// ctxTokenKey is used to pull a pre-issued JWT out of context, when the
// caller already resolved one upstream (e.g. request-scoped token reuse).
type ctxTokenKey struct{}

// WithJWT returns a context carrying an explicit JWT, bypassing cache/generation.
func WithJWT(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxTokenKey{}, token)
}

func tokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(ctxTokenKey{}).(string)
	return token, ok && token != ""
}

// renewBefore defines how far ahead of expiry we proactively rotate the
// cached token, so callers never hand out a token that expires mid-flight.
const renewBefore = 2 * time.Minute

// TokenCache holds an in-memory, thread-safe cache for a single signing
// identity's JWT, with request coalescing for concurrent generation.
type TokenCache struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time

	group singleflight.Group

	kid string
}

// NewTokenCache creates a TokenCache.
func NewTokenCache() *TokenCache {
	return &TokenCache{}
}

// get returns the cached token if it's still valid outside the renewal
// window, or "", false if it needs to be (re)generated.
func (c *TokenCache) get() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.token == "" || time.Now().After(c.expiresAt.Add(-renewBefore)) {
		return "", false
	}
	return c.token, true
}

func (c *TokenCache) set(token string, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.token = token
	c.expiresAt = expiresAt
}

// TokenGenerator resolves a JWT using, in order:
//  1. A token explicitly attached to ctx via WithJWT.
//  2. A cached token, if not near expiration.
//  3. A freshly generated token (deduplicated across concurrent callers).
func (c *TokenCache) TokenGenerator(
	ctx context.Context,
	appName string,
	cfg Config,
	kid string,
	keys map[string]string,
	l logger.Logger,
) (string, error) {
	if token, ok := tokenFromContext(ctx); ok {
		return token, nil
	}

	if token, ok := c.get(); ok {
		return token, nil
	}

	// singleflight ensures only one goroutine actually hits the signer
	// when many requests race in at once with a cold/expired cache.
	v, err, _ := c.group.Do("jwt", func() (interface{}, error) {
		// Re-check after acquiring the singleflight slot: another
		// goroutine may have just refreshed the cache.
		if token, ok := c.get(); ok {
			return token, nil
		}

		token, expiresAt, err := c.generate(appName, cfg, kid, keys)
		if err != nil {
			if l != nil {
				l.Error(ctx, "error while generating jwt token", logger.LogPayload{
					Type:   "INTERCEPTOR",
					Method: "TokenGenerator",
				})
			}
			return "", err
		}

		c.set(token, expiresAt)
		return token, nil
	})
	if err != nil {
		return "", err
	}

	return v.(string), nil
}

// generate issues a brand-new JWT and returns it along with its expiry.
func (c *TokenCache) generate(
	appName string,
	cfg Config,
	kid string,
	keys map[string]string,
) (string, time.Time, error) {
	b64Pvk, ok := keys[kid]
	if !ok {
		return "", time.Time{}, fmt.Errorf("jwt_generator: private key not found")
	}

	pvk, err := b64ToPrivateKey(b64Pvk)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt_generator: decode private key: %w", err)
	}

	ttl := time.Minute * 60
	expiresAt := time.Now().Add(ttl)

	extraClaims := map[string]interface{}{
		"token_type": "m2m",
		"client_id":  appName,
		"scope":      "internal:service",
		"aud":        "partner-service",
	}

	token, err := Issue(
		cfg,
		pvk,
		appName,
		"",
		ttl,
		extraClaims,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt_generator: issue token: %w", err)
	}

	return token, expiresAt, nil
}

func b64ToPrivateKey(b64PrivateKey string) (*rsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("jwt: decode rsa_private_key_base64: %w", err)
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse rsa_private_key_base64: %w", err)
	}

	key, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("jwt: rsa_private_key_base64 is not an RSA private key")
	}

	return key, nil
}
