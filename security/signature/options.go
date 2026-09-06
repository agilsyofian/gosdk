package signature

import (
	"time"
)

// Config defines the configuration for request verification and response signing.
type Config struct {
	SecretProvider          SecretProvider
	Algorithm               Algorithm
	Encoding                Encoding
	Canonicalizer           Canonicalizer
	ResponseCanonicalizer   ResponseCanonicalizer
	SignatureHeader         string
	TimestampHeader         string
	KeyIDHeader             string
	MaxClockDrift           time.Duration
	SignResponse            bool
	ResponseSignatureHeader string
	ResponseTimestampHeader string
}

// Option configures signature verification and signing behavior.
type Option func(*Config)

// WithAlgorithm sets the cryptographic algorithm (e.g. HMAC-SHA256, HMAC-SHA512, RSA-SHA256).
func WithAlgorithm(algo Algorithm) Option {
	return func(c *Config) {
		c.Algorithm = algo
	}
}

// WithEncoding sets the signature encoding format (Hex or Base64).
func WithEncoding(enc Encoding) Option {
	return func(c *Config) {
		c.Encoding = enc
	}
}

// WithCanonicalizer sets custom request canonicalization strategy.
func WithCanonicalizer(canonicalizer Canonicalizer) Option {
	return func(c *Config) {
		if canonicalizer != nil {
			c.Canonicalizer = canonicalizer
		}
	}
}

// WithResponseCanonicalizer sets custom response canonicalization strategy.
func WithResponseCanonicalizer(respCanonicalizer ResponseCanonicalizer) Option {
	return func(c *Config) {
		if respCanonicalizer != nil {
			c.ResponseCanonicalizer = respCanonicalizer
		}
	}
}

// WithHeaders sets the header names used for signature, timestamp, and key ID.
func WithHeaders(sigHeader, tsHeader, keyIDHeader string) Option {
	return func(c *Config) {
		if sigHeader != "" {
			c.SignatureHeader = sigHeader
		}
		if tsHeader != "" {
			c.TimestampHeader = tsHeader
		}
		if keyIDHeader != "" {
			c.KeyIDHeader = keyIDHeader
		}
	}
}

// WithMaxClockDrift sets maximum acceptable timestamp drift before a request is rejected.
func WithMaxClockDrift(drift time.Duration) Option {
	return func(c *Config) {
		c.MaxClockDrift = drift
	}
}

// WithSignResponse enables or disables signing outgoing HTTP responses.
func WithSignResponse(enabled bool) Option {
	return func(c *Config) {
		c.SignResponse = enabled
	}
}

// WithResponseHeaders sets header names for the outgoing response signature and timestamp.
func WithResponseHeaders(sigHeader, tsHeader string) Option {
	return func(c *Config) {
		if sigHeader != "" {
			c.ResponseSignatureHeader = sigHeader
		}
		if tsHeader != "" {
			c.ResponseTimestampHeader = tsHeader
		}
	}
}

// DefaultConfig returns standard signature configuration.
func DefaultConfig(provider SecretProvider) Config {
	return Config{
		SecretProvider:          provider,
		Algorithm:               AlgorithmHMACSHA256,
		Encoding:                EncodingHex,
		Canonicalizer:           NewDefaultCanonicalizer(),
		ResponseCanonicalizer:   NewDefaultResponseCanonicalizer(),
		SignatureHeader:         "X-Signature",
		TimestampHeader:         "X-Timestamp",
		KeyIDHeader:             "X-Client-Id",
		MaxClockDrift:           5 * time.Minute,
		SignResponse:            true,
		ResponseSignatureHeader: "X-Signature",
		ResponseTimestampHeader: "X-Timestamp",
	}
}
