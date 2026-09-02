// Package cryptoadapter bridges security/jwks.Manager to the
// utils/crypto.PublicKeyFetcher interface so that a RemoteJWKSKeyProvider
// can fetch public encryption keys from a remote JWKS endpoint without
// making the utils/crypto module depend on security/* packages.
//
// Placement rationale: this adapter lives in the application-wiring layer
// (examples/app/lib) because it is the only place that naturally imports
// both modules. utils/crypto stays zero-dep; security/jwks stays focused
// on JWT-verification key management.
package cryptoadapter

import (
	"context"
	"crypto/rsa"
	"fmt"

	"github.com/agilsyofian/gosdk/security/jwks"
)

// JWKSManagerAdapter wraps a security/jwks.Manager and satisfies the
// utils/crypto.PublicKeyFetcher interface. Inject it into
// crypto.NewRemoteJWKSKeyProvider to give the crypto layer access to
// remote/cached public keys without coupling the two modules directly.
//
// Example wiring:
//
//	manager := jwks.NewJWKSManager(jwks.Config{
//	    JWKSURL:        "https://partner.example.com/.well-known/jwks.json",
//	    CacheTTL:       time.Hour,
//	    RefreshTimeout: 5 * time.Second,
//	}, logger)
//
//	fetcher := cryptoadapter.NewJWKSManagerAdapter(manager)
//	kp      := crypto.NewRemoteJWKSKeyProvider(fetcher)
//	kp.SetPrivateKey("enc_v1", myPrivKey)
//	kp.SetActive("enc_v1")
//
//	encryptor := crypto.NewEncryptor(kp)
type JWKSManagerAdapter struct {
	manager jwks.Manager
}

// NewJWKSManagerAdapter wraps manager. manager must not be nil.
func NewJWKSManagerAdapter(manager jwks.Manager) *JWKSManagerAdapter {
	if manager == nil {
		panic("cryptoadapter: NewJWKSManagerAdapter: manager must not be nil")
	}
	return &JWKSManagerAdapter{manager: manager}
}

// FetchPublicKey implements crypto.PublicKeyFetcher. It delegates to the
// underlying jwks.Manager, which handles caching, singleflight dedup,
// and retry/circuit-breaker — all transparent to the crypto layer.
func (a *JWKSManagerAdapter) FetchPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	pub, err := a.manager.GetKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("cryptoadapter: fetch public key for kid %q: %w", kid, err)
	}
	return pub, nil
}
