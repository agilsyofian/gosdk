package jwks

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/resilience/circuitbreaker"
	"github.com/agilsyofian/gosdk/core/resilience/retry"
	"github.com/agilsyofian/gosdk/transport/client/http"
	"github.com/agilsyofian/gosdk/utils/crypto"
	"golang.org/x/sync/singleflight"
)

// Manager handles fetching and caching JSON Web Key Sets.
type Manager interface {
	GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error)
	Refresh(ctx context.Context) error
}

// Key aliases crypto.Key to maintain a single source of truth for JWK definitions.
type Key = crypto.Key

// Jwks aliases crypto.Jwks to maintain a single source of truth for JWK sets.
type Jwks = crypto.Jwks

type defaultJWKSManager struct {
	cfg        Config
	httpClient *http.Client

	mu     sync.RWMutex
	keys   map[string]*rsa.PublicKey
	expiry time.Time
	sf     singleflight.Group
}

func NewJWKSManager(cfg Config, logger logger.Logger) Manager {
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = time.Hour
	}
	if cfg.RefreshTimeout <= 0 {
		cfg.RefreshTimeout = 5 * time.Second
	}

	client := http.New(cfg.JWKSURL, cfg.RefreshTimeout, logger, http.Config{
		ServerName: "jwks-provider-service",
		BaseURL:    cfg.JWKSURL,
		Timeout:    0,
		Retry: retry.Policy{
			MaxAttempts: 3,
			Delay:       1 * time.Second,
		},
		CircuitBreaker: circuitbreaker.Config{
			Name:        "jwks-provider-cb",
			MinRequests: 10,
			MaxRequests: 2,
			Interval:    10 * time.Second,
			Timeout:     10 * time.Second,
			FailureRate: 0.5,
		},
	})

	return &defaultJWKSManager{
		cfg:        cfg,
		httpClient: client,
		keys:       make(map[string]*rsa.PublicKey),
	}
}

func (m *defaultJWKSManager) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	m.mu.RLock()
	key, ok := m.keys[kid]
	expired := time.Now().After(m.expiry)
	m.mu.RUnlock()

	if ok && !expired {
		return key, nil
	}

	// Not found or expired, trigger refresh
	if err := m.Refresh(ctx); err != nil {
		if !ok {
			return nil, err
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	key, ok = m.keys[kid]
	if !ok {
		return nil, fmt.Errorf("jwt: key with kid %q not found in JWKS", kid)
	}
	return key, nil
}

func (m *defaultJWKSManager) Refresh(ctx context.Context) error {
	_, err, _ := m.sf.Do("refresh", func() (any, error) {
		return nil, m.doRefresh(ctx)
	})
	return err
}

func (m *defaultJWKSManager) doRefresh(ctx context.Context) error {
	if m.cfg.JWKSURL == "" {
		return errors.New("jwt: JWKSURL is not configured")
	}

	var jwks Jwks
	err := m.httpClient.Execute(ctx, "GET", "", nil, &jwks)
	if err != nil {
		return fmt.Errorf("jwt: failed to fetch JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pubKey, err := decodeJWKToRSAPublicKey(k)
		if err != nil {
			continue
		}
		newKeys[k.Kid] = pubKey
	}

	m.mu.Lock()
	m.keys = newKeys
	m.expiry = time.Now().Add(m.cfg.CacheTTL)
	m.mu.Unlock()

	return nil
}

func decodeJWKToRSAPublicKey(k Key) (*rsa.PublicKey, error) {
	return crypto.JWKToPublicKey(k)
}
