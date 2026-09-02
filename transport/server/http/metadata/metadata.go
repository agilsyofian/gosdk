package metadata

import (
	"context"
	"net/http"
)

// Metadata constants
const (
	Authorization = "authorization"
	TraceID       = "x-trace-id"
	SpanID        = "x-span-id"
	SessionID     = "x-session-id"
	Nonce         = "x-nonce"
	Timestamp     = "x-timestamp"

	SignatureEnabled = "x-signature-enabled"
	Signature        = "x-signature"

	DeviceID         = "x-device-id"
	DeviceOs         = "x-device-os"
	DeviceAppVersion = "x-app-version"
	DeviceAppBuild   = "x-app-build"
	DeviceBrand      = "x-device-brand"
	DeviceInfo       = "x-device-info"

	// Field-level encryption headers per encryption-steerfile.md §12.
	EncryptionVersion = "x-encryption-version"
	EncryptionKid     = "x-encryption-kid"
	EncryptionKey     = "x-encryption-key"
)

// Request contains the supported HTTP request metadata. It deliberately
// excludes arbitrary headers so application code has an explicit, stable
// contract for the values it consumes.
type Request struct {
	Authorization    string
	TraceID          string
	SpanID           string
	SessionID        string
	Nonce            string
	Timestamp        string
	SignatureEnabled string
	Signature        string
	DeviceID         string
	DeviceOS         string
	DeviceAppVersion string
	DeviceAppBuild   string
	DeviceBrand      string
	DeviceInfo       string
	// Field-level encryption headers per encryption-steerfile.md §12.
	EncryptionVersion string
	EncryptionKid     string
	EncryptionKey     string
}

type contextKey struct{}

// FromHeader extracts the supported metadata from an HTTP header. Header.Get
// is case-insensitive, as required by HTTP.
func FromHeader(header http.Header) Request {
	return Request{
		Authorization:     header.Get(Authorization),
		TraceID:           header.Get(TraceID),
		SpanID:            header.Get(SpanID),
		SessionID:         header.Get(SessionID),
		Nonce:             header.Get(Nonce),
		Timestamp:         header.Get(Timestamp),
		SignatureEnabled:  header.Get(SignatureEnabled),
		Signature:         header.Get(Signature),
		DeviceID:          header.Get(DeviceID),
		DeviceOS:          header.Get(DeviceOs),
		DeviceAppVersion:  header.Get(DeviceAppVersion),
		DeviceAppBuild:    header.Get(DeviceAppBuild),
		DeviceBrand:       header.Get(DeviceBrand),
		DeviceInfo:        header.Get(DeviceInfo),
		EncryptionVersion: header.Get(EncryptionVersion),
		EncryptionKid:     header.Get(EncryptionKid),
		EncryptionKey:     header.Get(EncryptionKey),
	}
}

// WithContext attaches request metadata to ctx.
func WithContext(ctx context.Context, request Request) context.Context {
	return context.WithValue(ctx, contextKey{}, request)
}

// FromContext retrieves request metadata. The returned boolean is false when
// the context was not created by the HTTP metadata middleware.
func FromContext(ctx context.Context) (Request, bool) {
	request, ok := ctx.Value(contextKey{}).(Request)
	return request, ok
}
