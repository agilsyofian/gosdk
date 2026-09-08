package security

import (
	"context"
	"crypto/rsa"
	"fmt"

	"user-service/app/config"

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

// FieldDecryptor initializes a crypto.FieldDecryptor using a remote JWKS provider.
// It dynamically fetches public keys from jwks_url and uses the local private key for decryption.
func FieldDecryptor(cfg config.AppConfig, l logger.Logger) (*crypto.FieldDecryptor, error) {
	encKid := cfg.Security.Kid.Encryption

	jwksURL := cfg.Security.JWKSURL
	if jwksURL == "" {
		jwksURL = cfg.Security.Authenticators.Jwt.JWKSURL
	}
	if jwksURL == "" {
		return nil, fmt.Errorf("crypto: jwks_url is not configured")
	}

	cacheTTL := cfg.Security.Authenticators.Jwt.CacheTTL
	refreshTimeout := cfg.Security.Authenticators.Jwt.RefreshTimeout

	jwksMgr := jwks.NewJWKSManager(jwks.Config{
		JWKSURL:        jwksURL,
		CacheTTL:       cacheTTL,
		RefreshTimeout: refreshTimeout,
	}, l)

	kp := crypto.NewRemoteJWKSKeyProvider(&jwksFetcherAdapter{manager: jwksMgr})

	if privB64, ok := cfg.Security.PrivateKey[encKid]; ok && privB64 != "" {
		privKey, err := jwt.B64ToPrivateKey(privB64)
		if err != nil {
			return nil, fmt.Errorf("crypto: parse encryption private key for kid %q: %w", encKid, err)
		}
		if err := kp.SetPrivateKey(encKid, privKey); err != nil {
			return nil, fmt.Errorf("crypto: set encryption private key: %w", err)
		}
		if err := kp.SetActive(encKid); err != nil {
			return nil, fmt.Errorf("crypto: set active encryption key: %w", err)
		}
	}

	return crypto.NewFieldDecryptor(kp), nil
}
