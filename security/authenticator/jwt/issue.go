package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func Issue(cfg Config, pvk *rsa.PrivateKey, subject, sessionID string, ttl time.Duration, extraClaims map[string]any) (string, error) {
	claims := buildClaims(cfg, subject, sessionID, ttl, extraClaims)
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": cfg.Kid}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("jwt: encode jwt header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: encode jwt claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, pvk, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("jwt: sign jwt: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func buildClaims(cfg Config, subject, sessionID string, ttl time.Duration, extraClaims map[string]any) map[string]any {
	now := time.Now()
	claims := map[string]any{
		"sub": subject,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	if sessionID != "" {
		claims["sid"] = sessionID
	}
	if cfg.Issuer != "" {
		claims["iss"] = cfg.Issuer
	}
	if cfg.Audience != "" {
		claims["aud"] = cfg.Audience
	}
	for k, v := range extraClaims {
		switch k {
		case "sub", "sid", "iat", "exp", "iss", "aud":
			continue // never let caller-supplied claims override the reserved ones above
		}
		claims[k] = v
	}
	return claims
}

func B64ToPrivateKey(b64PrivateKey string) (*rsa.PrivateKey, error) {
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
