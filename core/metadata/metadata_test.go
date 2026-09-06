package metadata_test

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/metadata"
	"github.com/stretchr/testify/assert"
)

func TestMetadata_GetAndSet(t *testing.T) {
	m := metadata.New()
	m.Set("X-Custom-Tenant", "tenant-123")
	m.Set("x-device-id", "device-abc")

	assert.Equal(t, "tenant-123", m.Get("X-Custom-Tenant"))
	assert.Equal(t, "tenant-123", m.Get("x-custom-tenant"))
	assert.Equal(t, "device-abc", m.Get("X-Device-Id"))
	assert.Equal(t, "", m.Get("non-existent"))

	// Standard fields
	m.Set("x-trace-id", "trace-999")
	assert.Equal(t, "trace-999", m.TraceID)
	assert.Equal(t, "trace-999", m.Get("x-trace-id"))
	assert.Equal(t, "trace-999", m.Get("trace_id"))
}

func TestMetadata_GetAll(t *testing.T) {
	m := metadata.New()
	m.TraceID = "trace-1"
	m.UserID = "user-1"
	m.Set("custom-key", "custom-val")

	all := m.GetAll()
	assert.Equal(t, "trace-1", all["trace_id"])
	assert.Equal(t, "user-1", all["user_id"])
	assert.Equal(t, "custom-val", all["custom-key"])
}

func TestMetadata_Clone(t *testing.T) {
	m := metadata.New()
	m.TraceID = "trace-1"
	m.Set("key", "val")

	clone := m.Clone()
	assert.Equal(t, m.TraceID, clone.TraceID)
	assert.Equal(t, m.Get("key"), clone.Get("key"))

	clone.Set("key", "val-changed")
	assert.Equal(t, "val", m.Get("key"))
	assert.Equal(t, "val-changed", clone.Get("key"))
}

func TestMetadata_Context(t *testing.T) {
	m := metadata.New()
	m.TraceID = "trace-123"
	m.Set("x-company-code", "COMP-A")

	ctx := metadata.WithContext(context.Background(), m)

	extracted, ok := metadata.FromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "trace-123", extracted.TraceID)
	assert.Equal(t, "COMP-A", extracted.Get("x-company-code"))

	assert.Equal(t, "COMP-A", metadata.Value(ctx, "x-company-code"))
	assert.Equal(t, "", metadata.Value(ctx, "non-existent"))
	assert.Equal(t, "", metadata.Value(context.Background(), "x-company-code"))
}

func TestExtract_CompanyACustomMapping(t *testing.T) {
	carrier := metadata.MapCarrier{
		"Traceparent":      "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"X-Correlation-Id": "req-9876",
		"X-App-Tenant":     "tenant-fintech",
		"X-Custom-Client":  "client-mobile-v2",
	}

	cfg := metadata.Config{
		TraceIDHeader:   "Traceparent",
		RequestIDHeader: "X-Correlation-Id",
		CaptureHeaders:  []string{"X-App-Tenant"},
		CapturePrefixes: []string{"X-Custom-"},
	}

	meta := metadata.Extract(carrier, cfg)

	// Traceparent extracted 32-char trace ID
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", meta.TraceID)
	assert.Equal(t, "req-9876", meta.RequestID)
	assert.Equal(t, "tenant-fintech", meta.Get("x-app-tenant"))
	assert.Equal(t, "client-mobile-v2", meta.Get("x-custom-client"))
}

func TestExtract_DefaultFintechHeaders(t *testing.T) {
	carrier := metadata.MapCarrier{
		"X-Trace-Id":       "trace-111",
		"X-Device-Id":      "device-999",
		"X-App-Version":    "2.4.1",
		"X-Encryption-Kid": "kid-001",
		"X-Signature":      "sig-abc",
		"Authorization":    "Bearer token123",
	}

	meta := metadata.Extract(carrier)

	assert.Equal(t, "trace-111", meta.TraceID)
	assert.Equal(t, "Bearer token123", meta.Authorization)
	assert.Equal(t, "device-999", meta.Get("x-device-id"))
	assert.Equal(t, "2.4.1", meta.Get("x-app-version"))
	assert.Equal(t, "kid-001", meta.Get("x-encryption-kid"))
	assert.Equal(t, "sig-abc", meta.Get("x-signature"))
}
