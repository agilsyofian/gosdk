package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"time"
)

// Decryptor implements the backend decryption flow described in
// STEERFILE.md §11: parse -> validate policy -> validate freshness/replay
// -> unwrap key -> reconstruct AAD -> AES-GCM open. Cryptographic
// authentication always happens before the returned plaintext is
// trusted by callers, per STEERFILE.md §11.
type Decryptor struct {
	keys   KeyProvider
	replay ReplayStore
	policy ReplayPolicy

	maxEnvelopeBytes  int
	maxPlaintextBytes int
	nowFunc           func() time.Time

	// requireReplayProtection, when true, makes decryption fail closed
	// if no ReplayStore was configured. Set false only for endpoints
	// STEERFILE.md §10 does not classify as sensitive/state-changing.
	requireReplayProtection bool
}

// DecryptorOption configures a Decryptor built by NewDecryptor.
type DecryptorOption func(*Decryptor)

// WithReplayStore enables replay detection using store, and optionally
// overrides the default ReplayPolicy.
func WithReplayStore(store ReplayStore, policy ReplayPolicy) DecryptorOption {
	return func(d *Decryptor) {
		d.replay = store
		d.policy = policy.withDefaults()
	}
}

// WithoutReplayProtection explicitly opts out of replay protection, for
// endpoints STEERFILE.md §10 does not classify as sensitive. Prefer
// WithReplayStore for anything state-changing.
func WithoutReplayProtection() DecryptorOption {
	return func(d *Decryptor) { d.requireReplayProtection = false }
}

// WithMaxEnvelopeBytes overrides DefaultMaxEnvelopeBytes.
func WithMaxEnvelopeBytes(n int) DecryptorOption {
	return func(d *Decryptor) { d.maxEnvelopeBytes = n }
}

// WithMaxPlaintextBytes overrides DefaultMaxPlaintextBytes.
func WithMaxPlaintextBytes(n int) DecryptorOption {
	return func(d *Decryptor) { d.maxPlaintextBytes = n }
}

// NewDecryptor builds a Decryptor backed by keys. Replay protection is
// REQUIRED by default (STEERFILE.md §10); pass WithoutReplayProtection
// explicitly for endpoints that don't need it, or WithReplayStore to
// supply the store.
func NewDecryptor(keys KeyProvider, opts ...DecryptorOption) *Decryptor {
	d := &Decryptor{
		keys:                    keys,
		policy:                  DefaultReplayPolicy,
		maxEnvelopeBytes:        DefaultMaxEnvelopeBytes,
		maxPlaintextBytes:       DefaultMaxPlaintextBytes,
		nowFunc:                 time.Now,
		requireReplayProtection: true,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// acceptedVersions/acceptedAlgorithms are the protocol surface this
// Decryptor build understands. Kept as package-level slices so adding a
// new protocol version is a one-line, reviewable change rather than
// scattered conditionals.
var acceptedVersions = map[string]bool{ProtocolVersion1: true}
var acceptedAlgorithms = map[string]bool{AlgorithmA256GCM: true}

// Decrypt runs the full backend decryption flow against raw envelope
// bytes (e.g. an HTTP request body) and returns authenticated plaintext.
// meta MUST be built by the caller from trusted request context (HTTP
// method, path, content type, tenant) — never from attacker-controlled
// envelope fields — since it forms half of the AAD contract with the
// frontend.
//
// meta.RequestID and meta.KeyID are filled in from the parsed envelope
// automatically; callers only need to set Version (or leave it, it's
// overwritten to the parsed value for AAD purposes after version
// validation), Method, Path, ContentType, and TenantID.
func (d *Decryptor) Decrypt(ctx context.Context, raw []byte, meta Metadata) ([]byte, error) {
	env, err := ParseEnvelope(raw, d.maxEnvelopeBytes)
	if err != nil {
		return nil, err
	}
	return d.DecryptEnvelope(ctx, env, meta)
}

// DecryptEnvelope runs the flow against an already-parsed Envelope, for
// callers that received the envelope as structured JSON (e.g. a JSON
// request body decoded by a framework) rather than raw bytes.
func (d *Decryptor) DecryptEnvelope(ctx context.Context, env *Envelope, meta Metadata) ([]byte, error) {
	plaintext, _, err := d.decryptEnvelope(ctx, env, meta)
	return plaintext, err
}

// DecryptForResponse behaves like DecryptEnvelope but additionally
// returns a Session that ResponseEncryptor can use to encrypt a reply
// on the same request without re-wrapping a key with RSA, per
// STEERFILE.md §13. Use this only when backend->frontend
// application-layer encryption is actually required; for ordinary APIs
// TLS alone is sufficient (STEERFILE.md §13).
func (d *Decryptor) DecryptForResponse(ctx context.Context, env *Envelope, meta Metadata) ([]byte, *Session, error) {
	return d.decryptEnvelope(ctx, env, meta)
}

func (d *Decryptor) decryptEnvelope(ctx context.Context, env *Envelope, meta Metadata) ([]byte, *Session, error) {
	// DecryptEnvelope/DecryptForResponse are public entry points that
	// may receive an Envelope built or mutated by the caller (e.g. from
	// a framework-decoded JSON body) rather than one that passed
	// through ParseEnvelope. Re-run structural validation here so a
	// caller can never skip field-length/format checks by bypassing
	// ParseEnvelope, per STEERFILE.md §20 (validate before decrypting).
	if err := env.validateStructure(); err != nil {
		return nil, nil, err
	}

	if !acceptedVersions[env.Version] {
		return nil, nil, newError(ErrCodeUnsupportedVersion, "version %q not accepted", env.Version)
	}
	if !acceptedAlgorithms[env.Algorithm] {
		return nil, nil, newError(ErrCodeUnsupportedAlgorithm, "algorithm %q not accepted", env.Algorithm)
	}

	now := d.nowFunc()
	if err := d.policy.validateTimestamp(env.Timestamp, now); err != nil {
		return nil, nil, err
	}

	if d.replay != nil {
		seen, err := d.replay.CheckAndStore(ctx, env.RequestID, d.policy.withDefaults().Window)
		if err != nil {
			return nil, nil, newError(ErrCodeKeyProviderUnavailable, "replay store: %v", err)
		}
		if seen {
			return nil, nil, newError(ErrCodeReplayDetected, "request_id %q already processed", env.RequestID)
		}
	} else if d.requireReplayProtection {
		return nil, nil, newError(ErrCodeReplayDetected, "replay protection required but no ReplayStore configured")
	}

	privAny, err := d.keys.GetPrivateKey(ctx, env.KeyID)
	if err != nil {
		if _, ok := CodeOf(err); ok {
			return nil, nil, err
		}
		return nil, nil, newError(ErrCodeKeyProviderUnavailable, "key provider: %v", err)
	}
	priv, ok := privAny.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, newError(ErrCodeUnknownKeyID, "key_id %q is not an RSA key", env.KeyID)
	}

	wrappedKey, err := b64.DecodeString(env.EncryptedKey)
	if err != nil {
		return nil, nil, newError(ErrCodeMalformedEnvelope, "encrypted_key decode: %v", err)
	}
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, wrappedKey, nil)
	if err != nil {
		// Deliberately generic: do not distinguish OAEP padding
		// failure reasons, per STEERFILE.md §17.
		return nil, nil, newError(ErrCodeKeyUnwrapFailed, "rsa-oaep unwrap failed")
	}
	if len(aesKey) != aesKeySize {
		return nil, nil, newError(ErrCodeKeyUnwrapFailed, "unwrapped key length %d, want %d", len(aesKey), aesKeySize)
	}

	iv, err := b64.DecodeString(env.IV)
	if err != nil {
		return nil, nil, newError(ErrCodeMalformedEnvelope, "iv decode: %v", err)
	}
	ciphertextAndTag, err := decodeCiphertextAndTag(env)
	if err != nil {
		return nil, nil, err
	}

	// AAD is reconstructed from trusted server-side context, not
	// echoed from the envelope, per STEERFILE.md §9.
	meta.Version = env.Version
	meta.KeyID = env.KeyID
	meta.RequestID = env.RequestID
	aad := buildAAD(meta)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, nil, newError(ErrCodeAuthenticationFailed, "aes cipher init: %v", err)
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, aesGCMNonceSize)
	if err != nil {
		return nil, nil, newError(ErrCodeAuthenticationFailed, "gcm init: %v", err)
	}

	plaintext, err := gcm.Open(nil, iv, ciphertextAndTag, aad)
	if err != nil {
		// Generic message: never reveal whether it was the tag, the
		// ciphertext, or the AAD that mismatched, per STEERFILE.md §17.
		return nil, nil, newError(ErrCodeAuthenticationFailed, "gcm authentication failed")
	}
	if len(plaintext) > d.maxPlaintextBytes {
		return nil, nil, newError(ErrCodePayloadTooLarge, "plaintext size %d exceeds max %d", len(plaintext), d.maxPlaintextBytes)
	}
	session := &Session{ResponseKey: deriveResponseKey(aesKey, env.RequestID)}
	return plaintext, session, nil
}

// decodeCiphertextAndTag returns ciphertext||tag as Go's crypto/cipher
// AEAD expects. If the envelope represents the tag separately (Tag !=
// ""), it appends it; otherwise the caller has already validated that
// Ciphertext is long enough to contain a trailing tag itself.
func decodeCiphertextAndTag(env *Envelope) ([]byte, error) {
	ct, err := b64.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "ciphertext decode: %v", err)
	}
	if env.Tag == "" {
		return ct, nil
	}
	tag, err := b64.DecodeString(env.Tag)
	if err != nil {
		return nil, newError(ErrCodeMalformedEnvelope, "tag decode: %v", err)
	}
	return append(ct, tag...), nil
}
