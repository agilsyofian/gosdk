package http

import (
	"time"

	"github.com/agilsyofian/gosdk/security/signature"
	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
)

// SignatureSecretProvider defines the secret provider interface.
type SignatureSecretProvider = signature.SecretProvider

// SignatureOption defines functional options for configuring signature verification.
type SignatureOption = signature.Option

// Signature returns a Gin middleware ensuring HMAC/digital signature verification and response signing.
func Signature(provider SignatureSecretProvider, opts ...SignatureOption) gin.HandlerFunc {
	return middleware.Signature(provider, opts...)
}

// WithSignatureAlgorithm sets the signature algorithm (e.g. HMAC-SHA256, HMAC-SHA512, RSA-SHA256).
func WithSignatureAlgorithm(algo signature.Algorithm) SignatureOption {
	return signature.WithAlgorithm(algo)
}

// WithSignatureEncoding sets signature encoding (Hex or Base64).
func WithSignatureEncoding(enc signature.Encoding) SignatureOption {
	return signature.WithEncoding(enc)
}

// WithSignatureCanonicalizer sets custom request canonicalization strategy.
func WithSignatureCanonicalizer(c signature.Canonicalizer) SignatureOption {
	return signature.WithCanonicalizer(c)
}

// WithSignatureHeaders sets the header names for signature, timestamp, and key ID.
func WithSignatureHeaders(sigHeader, tsHeader, keyIDHeader string) SignatureOption {
	return signature.WithHeaders(sigHeader, tsHeader, keyIDHeader)
}

// WithSignatureMaxClockDrift sets maximum allowed timestamp skew.
func WithSignatureMaxClockDrift(drift time.Duration) SignatureOption {
	return signature.WithMaxClockDrift(drift)
}

// WithSignatureResponseSigning enables or disables outgoing response signing.
func WithSignatureResponseSigning(enabled bool) SignatureOption {
	return signature.WithSignResponse(enabled)
}

// StaticSecret creates a static SecretProvider.
func StaticSecret(secret []byte) SignatureSecretProvider {
	return signature.StaticSecret(secret)
}

// MapSecret creates a map-backed SecretProvider.
func MapSecret(secrets map[string][]byte) SignatureSecretProvider {
	return signature.MapSecret(secrets)
}
