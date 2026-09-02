package crypto

import (
	"context"
	"crypto"
	"crypto/rsa"
	"sync"
)

// KeyProvider abstracts backend key-pair access so this package is never
// coupled to a single secret-management product, per STEERFILE.md §15.
// Production implementations SHOULD wrap a Cloud KMS, HSM, Vault, or
// other approved key-management infrastructure. Implementations MUST
// keep private key material outside source code and ordinary
// application configuration where possible, and MUST NOT log it.
type KeyProvider interface {
	// GetPrivateKey resolves keyID to the private key used to unwrap
	// AES keys during decryption. It MUST return ErrCodeUnknownKeyID
	// (via a *Error) for an unrecognized or revoked keyID, without
	// distinguishing "never existed" from "revoked" in the response.
	GetPrivateKey(ctx context.Context, keyID string) (crypto.PrivateKey, error)

	// GetPublicKey resolves keyID to the public key frontends should
	// wrap AES keys with. Used by backend-authored test vectors and by
	// any server-side key-distribution endpoint; not required for
	// Decrypt itself.
	GetPublicKey(ctx context.Context, keyID string) (crypto.PublicKey, error)

	// ActiveKeyID returns the key_id new encryptions should use. This
	// lets Encrypt/response-signing pick the current key without the
	// caller hard-coding it, supporting rotation per STEERFILE.md §16.
	ActiveKeyID(ctx context.Context) (string, error)
}

// PublicKeyFetcher is a narrow interface for dynamically resolving a
// public key by kid from any remote or cacheable source (e.g. a JWKS
// URL, a KMS, or a sidecar). It is intentionally minimal so that
// utils/crypto remains zero-dependency — the concrete implementation
// (e.g. an adapter wrapping security/jwks.Manager) lives outside this
// module and is supplied by the caller via RemoteJWKSKeyProvider.
//
// Implementations MUST be safe for concurrent use, SHOULD cache their
// results, and SHOULD return a *Error with ErrCodeUnknownKeyID for an
// unrecognised kid.
type PublicKeyFetcher interface {
	FetchPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error)
}

// StaticKeyProvider is a reference, in-memory KeyProvider intended for
// tests, local development, and interoperability test vectors. It is
// NOT a production KeyProvider: production deployments MUST supply an
// implementation backed by a KMS/HSM/secret manager per STEERFILE.md §15.
type StaticKeyProvider struct {
	mu     sync.RWMutex
	active string
	keys   map[string]*rsa.PrivateKey // includes retired keys, per §16
}

// NewStaticKeyProvider builds a StaticKeyProvider with no keys
// registered. Call AddKey to register the active key and any retired
// keys still accepted during a rotation window.
func NewStaticKeyProvider() *StaticKeyProvider {
	return &StaticKeyProvider{keys: make(map[string]*rsa.PrivateKey)}
}

// AddKey registers keyID -> priv. If active is true, keyID becomes the
// value returned by ActiveKeyID; registering a new active key does not
// remove previously registered (now-retired) keys, so Decrypt can keep
// accepting them during a migration window per STEERFILE.md §16.
func (p *StaticKeyProvider) AddKey(keyID string, priv *rsa.PrivateKey, active bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys[keyID] = priv
	if active {
		p.active = keyID
	}
}

// RevokeKey immediately removes keyID so future decryption attempts
// against it fail with ErrCodeUnknownKeyID, per STEERFILE.md §16
// ("compromised keys MUST be revocable immediately").
func (p *StaticKeyProvider) RevokeKey(keyID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.keys, keyID)
	if p.active == keyID {
		p.active = ""
	}
}

func (p *StaticKeyProvider) GetPrivateKey(_ context.Context, keyID string) (crypto.PrivateKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	k, ok := p.keys[keyID]
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "key_id %q not registered", keyID)
	}
	return k, nil
}

func (p *StaticKeyProvider) GetPublicKey(_ context.Context, keyID string) (crypto.PublicKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	k, ok := p.keys[keyID]
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "key_id %q not registered", keyID)
	}
	return &k.PublicKey, nil
}

func (p *StaticKeyProvider) ActiveKeyID(_ context.Context) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.active == "" {
		return "", newError(ErrCodeUnknownKeyID, "no active key configured")
	}
	return p.active, nil
}
