package jwt

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/security/jwks"
)

// Algorithm identifies the signing algorithm used for a JWT.
type Algorithm string

const (
	HS256 Algorithm = "HS256"
	RS256 Algorithm = "RS256"
)

// jwtHeader is the decoded JOSE header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

// Claims models the claims this SDK expects a JWT to carry.
//
// Per the recommended JWT design, a JWT should preferably contain
// only authentication-related claims (sub, sid, iat, exp, and
// optionally iss/aud/nbf) -- not roles, permissions, device info, or
// other dynamic user attributes, which should come from the
// server-side Session instead.
type Claims struct {
	Subject   string `json:"sub"`
	SessionID string `json:"sid,omitempty"`
	Issuer    string `json:"iss,omitempty"`
	Audience  string `json:"aud,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	ExpiresAt int64  `json:"exp"`
	NotBefore int64  `json:"nbf,omitempty"`

	// Extra holds any additional claims present on the token. Treat
	// these with the same skepticism as the rest of the JWT: they are
	// authentication-time data, not authoritative session state.
	Extra map[string]any `json:"-"`
}

// verifyConfig configures signature and claim validation.
type verifyConfig struct {
	// JWKS is used to retrieve public keys for RS256.
	JWKS jwks.Manager

	// Issuer, if non-empty, MUST match the token's "iss" claim.
	Issuer string

	// Audience, if non-empty, MUST match the token's "aud" claim.
	Audience string

	// Leeway allows for small clock skew when validating exp/nbf.
	Leeway time.Duration
}

var (
	errMalformedToken   = errors.New("jwt: malformed token")
	errUnsupportedAlg   = errors.New("jwt: unsupported or mismatched algorithm")
	errBadSignature     = errors.New("jwt: signature verification failed")
	errTokenExpired     = errors.New("jwt: token expired")
	errTokenNotYetValid = errors.New("jwt: token not yet valid")
	errIssuerMismatch   = errors.New("jwt: issuer mismatch")
	errAudienceMismatch = errors.New("jwt: audience mismatch")
)

// parseAndVerify validates the signature, expiration, not-before,
// issuer, and audience (as configured) of a compact-serialized JWT,
// and returns its claims.
//
// Signature validation always happens before any claim is trusted --
// callers must not read Subject/SessionID/etc from an unverified
// token.
func parseAndVerify(ctx context.Context, token string, cfg verifyConfig) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errMalformedToken
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	headerRaw, err := base64.RawURLEncoding.DecodeString(headerB64)
	if err != nil {
		return nil, fmt.Errorf("%w: header: %v", errMalformedToken, err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return nil, fmt.Errorf("%w: header: %v", errMalformedToken, err)
	}

	if !strings.EqualFold(header.Alg, string(RS256)) {
		return nil, errUnsupportedAlg
	}

	signature, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("%w: signature: %v", errMalformedToken, err)
	}

	signingInput := headerB64 + "." + payloadB64

	if cfg.JWKS == nil {
		return nil, errors.New("jwt: JWKS manager not configured")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	pubKey, err := cfg.JWKS.GetKey(ctx, header.Kid)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errBadSignature, err)
	}

	hashed := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed[:], signature); err != nil {
		return nil, errBadSignature
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("%w: payload: %v", errMalformedToken, err)
	}

	var claims Claims
	if err := json.Unmarshal(payloadRaw, &claims); err != nil {
		return nil, fmt.Errorf("%w: payload: %v", errMalformedToken, err)
	}

	var extra map[string]any
	if err := json.Unmarshal(payloadRaw, &extra); err == nil {
		claims.Extra = extra
	}

	now := time.Now()

	err = verify(claims, now, cfg)
	if err != nil {
		return nil, err
	}

	return &claims, nil
}

func verify(claims Claims, now time.Time, cfg verifyConfig) error {
	if claims.ExpiresAt == 0 {
		return fmt.Errorf("%w: missing exp", errMalformedToken)
	}
	if now.After(time.Unix(claims.ExpiresAt, 0).Add(cfg.Leeway)) {
		return errTokenExpired
	}
	if claims.NotBefore != 0 && now.Before(time.Unix(claims.NotBefore, 0).Add(-cfg.Leeway)) {
		return errTokenNotYetValid
	}
	if cfg.Issuer != "" && claims.Issuer != cfg.Issuer {
		return errIssuerMismatch
	}
	if cfg.Audience != "" && claims.Audience != cfg.Audience {
		return errAudienceMismatch
	}
	return nil
}
