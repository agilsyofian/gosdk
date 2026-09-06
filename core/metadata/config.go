package metadata

import "strings"

// Config defines declarative header mappings and dynamic capture rules
// to support company-specific naming conventions across HTTP and gRPC transports.
type Config struct {
	// Header name for Trace ID (default: "X-Trace-Id", fallback to "traceparent")
	TraceIDHeader string `mapstructure:"trace_id_header"`

	// Header name for Span ID (default: "X-Span-Id")
	SpanIDHeader string `mapstructure:"span_id_header"`

	// Header name for Request/Correlation ID (default: "X-Request-Id")
	RequestIDHeader string `mapstructure:"request_id_header"`

	// Header name for Session ID (default: "X-Session-Id")
	SessionIDHeader string `mapstructure:"session_id_header"`

	// Header name for User ID (default: "X-User-Id")
	UserIDHeader string `mapstructure:"user_id_header"`

	// Header name for Client IP (default: "X-Forwarded-For")
	ClientIPHeader string `mapstructure:"client_ip_header"`

	// Header name for Timestamp (default: "X-Timestamp")
	TimestampHeader string `mapstructure:"timestamp_header"`

	// Header name for Authorization (default: "Authorization")
	AuthHeader string `mapstructure:"auth_header"`

	// Explicit list of additional header names to capture into dynamic baggage
	CaptureHeaders []string `mapstructure:"capture_headers"`

	// Header prefixes to automatically capture (e.g. "x-")
	CapturePrefixes []string `mapstructure:"capture_prefixes"`
}

// DefaultConfig returns default metadata configuration adhering to industry standards
// with common fintech/mobile/enterprise headers pre-configured in capture rules.
func DefaultConfig() Config {
	return Config{
		TraceIDHeader:   "X-Trace-Id",
		SpanIDHeader:    "X-Span-Id",
		RequestIDHeader: "X-Request-Id",
		SessionIDHeader: "X-Session-Id",
		UserIDHeader:    "X-User-Id",
		ClientIPHeader:  "X-Forwarded-For",
		TimestampHeader: "X-Timestamp",
		AuthHeader:      "Authorization",
		CaptureHeaders: []string{
			// Common Identity & Session
			"x-nonce",
			"x-signature",
			"x-signature-enabled",
			// Device & Client Information
			"x-device-id",
			"x-device-os",
			"x-app-version",
			"x-app-build",
			"x-device-brand",
			"x-device-info",
			// Security & Encryption
			"x-encryption-version",
			"x-encryption-kid",
			"x-encryption-key",
			// Tenancy & Channel
			"x-tenant-id",
			"x-channel-id",
		},
		CapturePrefixes: []string{"x-"},
	}
}

// Normalize ensures empty fields are populated with defaults and strings are cleaned.
func (c Config) Normalize() Config {
	def := DefaultConfig()

	if strings.TrimSpace(c.TraceIDHeader) == "" {
		c.TraceIDHeader = def.TraceIDHeader
	}
	if strings.TrimSpace(c.SpanIDHeader) == "" {
		c.SpanIDHeader = def.SpanIDHeader
	}
	if strings.TrimSpace(c.RequestIDHeader) == "" {
		c.RequestIDHeader = def.RequestIDHeader
	}
	if strings.TrimSpace(c.SessionIDHeader) == "" {
		c.SessionIDHeader = def.SessionIDHeader
	}
	if strings.TrimSpace(c.UserIDHeader) == "" {
		c.UserIDHeader = def.UserIDHeader
	}
	if strings.TrimSpace(c.ClientIPHeader) == "" {
		c.ClientIPHeader = def.ClientIPHeader
	}
	if strings.TrimSpace(c.TimestampHeader) == "" {
		c.TimestampHeader = def.TimestampHeader
	}
	if strings.TrimSpace(c.AuthHeader) == "" {
		c.AuthHeader = def.AuthHeader
	}

	if len(c.CaptureHeaders) == 0 {
		c.CaptureHeaders = def.CaptureHeaders
	}
	if len(c.CapturePrefixes) == 0 {
		c.CapturePrefixes = def.CapturePrefixes
	}

	return c
}
