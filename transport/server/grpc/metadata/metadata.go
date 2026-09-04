package metadata

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"
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
	DeviceOS         = "x-device-os"
	DeviceAppVersion = "x-app-version"
	DeviceAppBuild   = "x-app-build"
	DeviceBrand      = "x-device-brand"
	DeviceInfo       = "x-device-info"

	EncryptionVersion = "x-encryption-version"
	EncryptionKid     = "x-encryption-kid"
	EncryptionKey     = "x-encryption-key"
)

type Request struct {
	Authorization     string
	TraceID           string
	SpanID            string
	SessionID         string
	Nonce             string
	Timestamp         string
	SignatureEnabled  string
	Signature         string
	DeviceID          string
	DeviceOS          string
	DeviceAppVersion  string
	DeviceAppBuild    string
	DeviceBrand       string
	DeviceInfo        string
	EncryptionVersion string
	EncryptionKid     string
	EncryptionKey     string
}

type contextKey struct{}

// FromMD extracts the supported metadata from gRPC incoming MD.
// gRPC normalises all metadata keys to lowercase, so lookup is done
// using lowercase versions of the Header values.
func FromMD(md metadata.MD) Request {
	return Request{
		Authorization:     first(md, Authorization),
		TraceID:           first(md, TraceID),
		SpanID:            first(md, SpanID),
		SessionID:         first(md, SessionID),
		Nonce:             first(md, Nonce),
		Timestamp:         first(md, Timestamp),
		SignatureEnabled:  first(md, SignatureEnabled),
		Signature:         first(md, Signature),
		DeviceID:          first(md, DeviceID),
		DeviceOS:          first(md, DeviceOS),
		DeviceAppVersion:  first(md, DeviceAppVersion),
		DeviceAppBuild:    first(md, DeviceAppBuild),
		DeviceBrand:       first(md, DeviceBrand),
		DeviceInfo:        first(md, DeviceInfo),
		EncryptionVersion: first(md, EncryptionVersion),
		EncryptionKid:     first(md, EncryptionKid),
		EncryptionKey:     first(md, EncryptionKey),
	}
}

// WithContext attaches request metadata to ctx.
func WithContext(ctx context.Context, req Request) context.Context {
	return context.WithValue(ctx, contextKey{}, req)
}

// FromContext retrieves request metadata from ctx.
func FromContext(ctx context.Context) (Request, bool) {
	req, ok := ctx.Value(contextKey{}).(Request)
	return req, ok
}

// first returns the first value for key in md, or "" if absent.
// It converts key to lowercase since gRPC incoming metadata keys are always lowercase.
func first(md metadata.MD, key string) string {
	vals := md.Get(strings.ToLower(key))
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
