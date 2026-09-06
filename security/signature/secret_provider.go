package signature

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrSecretNotFound = errors.New("signature: secret not found")
)

// SecretProvider resolves a secret or cryptographic key by keyID/clientID.
type SecretProvider interface {
	GetSecret(ctx context.Context, keyID string) ([]byte, error)
}

// StaticSecret returns a SecretProvider that always returns the same static secret.
func StaticSecret(secret []byte) SecretProvider {
	return &staticSecretProvider{secret: secret}
}

type staticSecretProvider struct {
	secret []byte
}

func (s *staticSecretProvider) GetSecret(_ context.Context, _ string) ([]byte, error) {
	return s.secret, nil
}

// MapSecret returns a SecretProvider backed by an in-memory map.
func MapSecret(secrets map[string][]byte) SecretProvider {
	cp := make(map[string][]byte, len(secrets))
	for k, v := range secrets {
		cp[k] = v
	}
	return &mapSecretProvider{secrets: cp}
}

type mapSecretProvider struct {
	mu      sync.RWMutex
	secrets map[string][]byte
}

func (m *mapSecretProvider) GetSecret(_ context.Context, keyID string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sec, ok := m.secrets[keyID]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return sec, nil
}

// FuncSecret returns a SecretProvider that delegates secret lookup to a function.
func FuncSecret(fn func(ctx context.Context, keyID string) ([]byte, error)) SecretProvider {
	return funcSecretProvider(fn)
}

type funcSecretProvider func(ctx context.Context, keyID string) ([]byte, error)

func (f funcSecretProvider) GetSecret(ctx context.Context, keyID string) ([]byte, error) {
	return f(ctx, keyID)
}
