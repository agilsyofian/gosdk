package security

import (
	"context"
	"crypto/rsa"
	"fmt"

	"auth-service/app/config"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/security/jwks"
	"github.com/agilsyofian/gosdk/utils/crypto"
)

// jwksFetcherAdapter adapts jwks.Manager to crypto.PublicKeyFetcher.
type jwksFetcherAdapter struct {
	manager jwks.Manager
}

func (a *jwksFetcherAdapter) FetchPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	return a.manager.GetKey(ctx, kid)
}

type keyConfigurable interface {
	crypto.KeyProvider
	SetPrivateKey(kid string, priv *rsa.PrivateKey) error
	SetActive(kid string) error
}

// FieldDecryptor initializes a crypto.FieldDecryptor.
// If jwks_url is configured, it dynamically fetches public keys from the remote JWKS endpoint.
// Otherwise, it falls back to local JWKS configuration.
func FieldDecryptor(cfg config.AppConfig, l logger.Logger) (*crypto.FieldDecryptor, error) {
	encKid := cfg.Security.Kid.Encryption
	kp, err := initKeyProvider(cfg, encKid, l)
	if err != nil {
		return nil, err
	}

	if err := configurePrivateKey(kp, cfg.Security.PrivateKey, encKid); err != nil {
		return nil, err
	}

	return crypto.NewFieldDecryptor(kp), nil
}

func initKeyProvider(cfg config.AppConfig, encKid string, l logger.Logger) (keyConfigurable, error) {
	if jwksURL := resolveJWKSURL(cfg); jwksURL != "" {
		return newRemoteKeyProvider(cfg, jwksURL, l), nil
	}
	return newLocalKeyProvider(cfg, encKid)
}

func resolveJWKSURL(cfg config.AppConfig) string {
	if cfg.Security.JWKSURL != "" {
		return cfg.Security.JWKSURL
	}
	return cfg.Security.Authenticators.Jwt.JWKSURL
}

func newRemoteKeyProvider(cfg config.AppConfig, jwksURL string, l logger.Logger) keyConfigurable {
	jwksMgr := jwks.NewJWKSManager(jwks.Config{
		JWKSURL:        jwksURL,
		CacheTTL:       cfg.Security.Authenticators.Jwt.CacheTTL,
		RefreshTimeout: cfg.Security.Authenticators.Jwt.RefreshTimeout,
	}, l)
	return crypto.NewRemoteJWKSKeyProvider(&jwksFetcherAdapter{manager: jwksMgr})
}

func newLocalKeyProvider(cfg config.AppConfig, encKid string) (keyConfigurable, error) {
	encKeys := filterEncryptionKeys(cfg.Security.JWKS.Keys, encKid)
	localKP, err := crypto.NewJWKSKeyProvider(crypto.Jwks{Keys: encKeys})
	if err != nil {
		return nil, fmt.Errorf("crypto: init jwks key provider: %w", err)
	}
	return localKP, nil
}

func filterEncryptionKeys(keys []crypto.Key, encKid string) []crypto.Key {
	var encKeys []crypto.Key
	for _, k := range keys {
		if k.Use == "enc" || k.Kid == encKid {
			encKeys = append(encKeys, crypto.Key{
				Use: k.Use,
				Kty: k.Kty,
				Alg: k.Alg,
				Kid: k.Kid,
				N:   k.N,
				E:   k.E,
				X5c: k.X5c,
			})
		}
	}
	return encKeys
}

func configurePrivateKey(kp keyConfigurable, privKeyMap map[string]string, encKid string) error {
	privB64, ok := privKeyMap[encKid]
	if !ok || privB64 == "" {
		return nil
	}

	privKey, err := jwt.B64ToPrivateKey(privB64)
	if err != nil {
		return fmt.Errorf("crypto: parse encryption private key for kid %q: %w", encKid, err)
	}
	if err := kp.SetPrivateKey(encKid, privKey); err != nil {
		return fmt.Errorf("crypto: set encryption private key: %w", err)
	}
	if err := kp.SetActive(encKid); err != nil {
		return fmt.Errorf("crypto: set active encryption key: %w", err)
	}
	return nil
}
