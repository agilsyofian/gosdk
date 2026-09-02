package crypto

import (
	"context"
	"crypto"
	"crypto/rsa"
	"fmt"
	"math/big"
	"sync"
)

// Key is one JWK entry, matching encryption-steerfile.md §3.1/§5
// exactly (field names, JSON tags, and mapstructure tags for Viper).
// Only public RSA parameters exist on this type by design — there is
// no field for d/p/q/etc., so a Key can never accidentally leak private
// key material through JSON marshaling, satisfying §4 ("Private
// parameters ... MUST NEVER be returned").
type Key struct {
	Use string   `json:"use" mapstructure:"use" validate:"required"`
	Kty string   `json:"kty" mapstructure:"kty" validate:"required"`
	Alg string   `json:"alg" mapstructure:"alg" validate:"required"`
	Kid string   `json:"kid" mapstructure:"kid" validate:"required"`
	N   string   `json:"n" mapstructure:"n" validate:"required"`
	E   string   `json:"e" mapstructure:"e" validate:"required"`
	X5c []string `json:"x5c" mapstructure:"x5c"`
}

// Jwks is the JWK Set, matching encryption-steerfile.md §4/§5. Loaded
// from YAML via Viper (`encryption.jwks`) and/or served verbatim as the
// body of GET /.well-known/jwks.json — the type is safe to marshal
// directly to either destination.
type Jwks struct {
	Keys []Key `json:"keys" mapstructure:"keys" validate:"required"`
}

// requiredUse and requiredAlg are the only values STEERFILE.md §3.2/§13
// allows for an encryption key. A Key failing either check is rejected
// at load time (NewJWKSKeyProvider) rather than deferred to first use,
// so a misconfigured key is caught at startup.
const (
	requiredKty = "RSA"
	requiredUse = "enc"
	requiredAlg = "RSA-OAEP-256"
)

// PublicKeyToJWK renders pub as a Key for publishing/config, encoding N
// and E as Base64URL without padding per STEERFILE.md §8.
func PublicKeyToJWK(pub *rsa.PublicKey, kid string) Key {
	return Key{
		Use: requiredUse,
		Kty: requiredKty,
		Alg: requiredAlg,
		Kid: kid,
		N:   b64.EncodeToString(pub.N.Bytes()),
		E:   b64.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		X5c: []string{},
	}
}

// JWKToPublicKey reconstructs an *rsa.PublicKey from a Key. It does NOT
// validate use/alg/kty — callers that need that validation (e.g. at
// JWKS load time) should check the fields directly first; this
// function is also used internally after that validation has already
// happened.
func JWKToPublicKey(k Key) (*rsa.PublicKey, error) {
	nBytes, err := b64.DecodeString(k.N)
	if err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "jwk.n: invalid base64url: %v", err)
	}
	eBytes, err := b64.DecodeString(k.E)
	if err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "jwk.e: invalid base64url: %v", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

// JWKSKeyProvider is a KeyProvider backed by a configured Jwks (public
// metadata + public key material, e.g. loaded via Viper from
// encryption.jwks per STEERFILE.md §5) plus private key material
// supplied separately via SetPrivateKey — exactly as STEERFILE.md §3.1
// describes ("private key MUST NOT be exposed through JWKS"). Where
// that private material comes from (a PEM file, Vault, a KMS reference)
// is a deployment decision outside this package's scope; wire it up at
// startup and call SetPrivateKey once per kid.
//
// JWKSKeyProvider validates kty/use/alg for every configured Key at
// load time (NewJWKSKeyProvider) and again defensively on every
// GetPrivateKey/GetPublicKey call, per STEERFILE.md §21 ("Validate
// kid... Validate encryption algorithm... Validate key usage").
type JWKSKeyProvider struct {
	mu     sync.RWMutex
	jwks   Jwks
	byKid  map[string]Key
	priv   map[string]*rsa.PrivateKey
	active string
}

// NewJWKSKeyProvider validates jwks and builds a JWKSKeyProvider from
// it. It returns an error (not a *Error — this is a startup/config
// failure, not a request-time cryptographic failure) if any Key has an
// unsupported kty/use/alg, a duplicate kid, or unparsable n/e.
func NewJWKSKeyProvider(jwks Jwks) (*JWKSKeyProvider, error) {
	byKid := make(map[string]Key, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kid == "" {
			return nil, fmt.Errorf("jwks: key missing kid")
		}
		if _, dup := byKid[k.Kid]; dup {
			return nil, fmt.Errorf("jwks: duplicate kid %q", k.Kid)
		}
		if k.Kty != requiredKty {
			return nil, fmt.Errorf("jwks: kid %q: unsupported kty %q, want %q", k.Kid, k.Kty, requiredKty)
		}
		if k.Use != requiredUse {
			return nil, fmt.Errorf("jwks: kid %q: unsupported use %q, want %q", k.Kid, k.Use, requiredUse)
		}
		if k.Alg != requiredAlg {
			return nil, fmt.Errorf("jwks: kid %q: unsupported alg %q, want %q", k.Kid, k.Alg, requiredAlg)
		}
		if _, err := JWKToPublicKey(k); err != nil {
			return nil, fmt.Errorf("jwks: kid %q: %v", k.Kid, err)
		}
		byKid[k.Kid] = k
	}
	return &JWKSKeyProvider{jwks: jwks, byKid: byKid, priv: make(map[string]*rsa.PrivateKey)}, nil
}

// SetPrivateKey attaches the private half of kid's key pair, sourced
// from wherever this deployment keeps private key material (never from
// JWKS/config). It fails if kid was not present in the Jwks passed to
// NewJWKSKeyProvider, and fails if priv's public key doesn't match the
// modulus/exponent already configured for kid — catching a
// misconfigured key pair at startup instead of at first decrypt.
func (p *JWKSKeyProvider) SetPrivateKey(kid string, priv *rsa.PrivateKey) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	k, ok := p.byKid[kid]
	if !ok {
		return fmt.Errorf("jwks: kid %q not present in configured jwks", kid)
	}
	pub, err := JWKToPublicKey(k)
	if err != nil {
		return err
	}
	if pub.N.Cmp(priv.PublicKey.N) != 0 || pub.E != priv.PublicKey.E {
		return fmt.Errorf("jwks: private key for kid %q does not match its configured public jwk", kid)
	}
	p.priv[kid] = priv
	return nil
}

// SetActive marks kid as the key new frontend/Postman encryptions
// should be told to use (i.e. the ENCRYPTION_KID value). It does not
// affect which keys DecryptFields will accept — every kid with an
// attached private key remains valid for decryption, which is what
// makes key rotation possible per STEERFILE.md §18.
func (p *JWKSKeyProvider) SetActive(kid string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.byKid[kid]; !ok {
		return fmt.Errorf("jwks: kid %q not present in configured jwks", kid)
	}
	p.active = kid
	return nil
}

// RevokeKey immediately removes kid's private key so future decryption
// attempts against it fail with ErrCodeUnknownKeyID, and removes it
// from the served JWKS so new encryptions can't target it either.
func (p *JWKSKeyProvider) RevokeKey(kid string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.priv, kid)
	delete(p.byKid, kid)
	if p.active == kid {
		p.active = ""
	}
}

func (p *JWKSKeyProvider) resolve(kid string) (Key, error) {
	k, ok := p.byKid[kid]
	if !ok {
		return Key{}, newError(ErrCodeUnknownKeyID, "kid %q not present in jwks", kid)
	}
	// Defensive re-check even though NewJWKSKeyProvider already
	// validated this — guards against any future path that mutates
	// byKid without going through that validation.
	if k.Kty != requiredKty || k.Use != requiredUse || k.Alg != requiredAlg {
		return Key{}, newError(ErrCodeUnsupportedAlgorithm, "kid %q: kty/use/alg not accepted", kid)
	}
	return k, nil
}

// GetPrivateKey implements KeyProvider.
func (p *JWKSKeyProvider) GetPrivateKey(_ context.Context, kid string) (crypto.PrivateKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if _, err := p.resolve(kid); err != nil {
		return nil, err
	}
	priv, ok := p.priv[kid]
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "kid %q has no private key configured", kid)
	}
	return priv, nil
}

// GetPublicKey implements KeyProvider.
func (p *JWKSKeyProvider) GetPublicKey(_ context.Context, kid string) (crypto.PublicKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	k, err := p.resolve(kid)
	if err != nil {
		return nil, err
	}
	return JWKToPublicKey(k)
}

// ActiveKeyID implements KeyProvider.
func (p *JWKSKeyProvider) ActiveKeyID(_ context.Context) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.active == "" {
		return "", newError(ErrCodeUnknownKeyID, "no active key configured")
	}
	return p.active, nil
}

// PublicJWKS returns the JWK Set exactly as it should be served from
// GET /.well-known/jwks.json (STEERFILE.md §4): public parameters only.
// Since Key has no private fields to begin with, this is simply the
// current (possibly revoked-key-trimmed) key list — wire it directly
// into a transport/http handler with json.Marshal.
func (p *JWKSKeyProvider) PublicJWKS() Jwks {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keys := make([]Key, 0, len(p.byKid))
	for _, k := range p.byKid {
		keys = append(keys, k)
	}
	return Jwks{Keys: keys}
}

// RemoteJWKSKeyProvider is a KeyProvider whose public keys come from a
// dynamic remote source (e.g. a JWKS URL) via an injected PublicKeyFetcher,
// while private keys are registered locally with SetPrivateKey — exactly
// matching STEERFILE.md §3.1 ("private key MUST NOT be exposed through JWKS").
//
// This type deliberately carries no HTTP/caching logic itself; all of
// that lives in the injected fetcher. This keeps utils/crypto zero-dep —
// the concrete fetcher (e.g. an adapter around security/jwks.Manager) is
// provided by the application wiring layer.
//
// Use-case: server B wants to encrypt a payload for server A without
// having A's public key baked into a YAML file. B uses a RemoteJWKSKeyProvider
// pointed at A's JWKS endpoint to fetch A's current encryption public key
// at startup (or on-demand), and sets its own private key locally for
// decryption.
type RemoteJWKSKeyProvider struct {
	mu      sync.RWMutex
	fetcher PublicKeyFetcher
	priv    map[string]*rsa.PrivateKey
	active  string
}

// NewRemoteJWKSKeyProvider creates a RemoteJWKSKeyProvider backed by
// fetcher. fetcher MUST NOT be nil. Private keys are added later via
// SetPrivateKey.
func NewRemoteJWKSKeyProvider(fetcher PublicKeyFetcher) *RemoteJWKSKeyProvider {
	if fetcher == nil {
		panic("crypto: NewRemoteJWKSKeyProvider: fetcher must not be nil")
	}
	return &RemoteJWKSKeyProvider{
		fetcher: fetcher,
		priv:    make(map[string]*rsa.PrivateKey),
	}
}

// SetPrivateKey registers the private key for kid. It is the caller's
// responsibility to ensure the private key matches the public key that
// the fetcher will return for that kid. kid must not be empty.
func (p *RemoteJWKSKeyProvider) SetPrivateKey(kid string, priv *rsa.PrivateKey) error {
	if kid == "" {
		return fmt.Errorf("crypto: RemoteJWKSKeyProvider.SetPrivateKey: kid must not be empty")
	}
	if priv == nil {
		return fmt.Errorf("crypto: RemoteJWKSKeyProvider.SetPrivateKey: priv must not be nil")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.priv[kid] = priv
	return nil
}

// SetActive marks kid as the key used by new encryptions (returned by
// ActiveKeyID). kid must have been registered via SetPrivateKey.
func (p *RemoteJWKSKeyProvider) SetActive(kid string) error {
	p.mu.RLock()
	_, ok := p.priv[kid]
	p.mu.RUnlock()
	if !ok {
		return fmt.Errorf("crypto: RemoteJWKSKeyProvider.SetActive: kid %q not registered via SetPrivateKey", kid)
	}
	p.mu.Lock()
	p.active = kid
	p.mu.Unlock()
	return nil
}

// RevokeKey immediately removes kid's private key so future decryption
// attempts against it fail with ErrCodeUnknownKeyID, supporting key
// rotation per STEERFILE.md §18.
func (p *RemoteJWKSKeyProvider) RevokeKey(kid string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.priv, kid)
	if p.active == kid {
		p.active = ""
	}
}

// GetPublicKey implements KeyProvider. It delegates to the injected fetcher
// so the caching/refresh strategy is owned entirely by the fetcher.
func (p *RemoteJWKSKeyProvider) GetPublicKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	pub, err := p.fetcher.FetchPublicKey(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("crypto: RemoteJWKSKeyProvider.GetPublicKey: %w", err)
	}
	return pub, nil
}

// GetPrivateKey implements KeyProvider. Private keys are always resolved
// from the local in-memory registry — never from a remote source.
func (p *RemoteJWKSKeyProvider) GetPrivateKey(_ context.Context, kid string) (crypto.PrivateKey, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	k, ok := p.priv[kid]
	if !ok {
		return nil, newError(ErrCodeUnknownKeyID, "kid %q has no private key registered", kid)
	}
	return k, nil
}

// ActiveKeyID implements KeyProvider.
func (p *RemoteJWKSKeyProvider) ActiveKeyID(_ context.Context) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.active == "" {
		return "", newError(ErrCodeUnknownKeyID, "no active key configured")
	}
	return p.active, nil
}
