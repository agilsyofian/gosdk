package metadata

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestFromMDAndContext(t *testing.T) {
	md := metadata.Pairs(
		DeviceID, "device-123",
		DeviceAppVersion, "1.2.3",
		"x-trace-id", "trace-abc",
		Authorization, "Bearer tok",
	)

	req := FromMD(md)

	if req.DeviceID != "device-123" {
		t.Errorf("DeviceId = %q, want %q", req.DeviceID, "device-123")
	}
	if req.DeviceAppVersion != "1.2.3" {
		t.Errorf("AppVersion = %q, want %q", req.DeviceAppVersion, "1.2.3")
	}
	if req.TraceID != "trace-abc" {
		t.Errorf("TraceId = %q, want %q", req.TraceID, "trace-abc")
	}
	if req.Authorization != "Bearer tok" {
		t.Errorf("Authorization = %q, want %q", req.Authorization, "Bearer tok")
	}

	ctx := WithContext(context.Background(), req)
	got, ok := FromContext(ctx)
	if !ok || got != req {
		t.Fatalf("FromContext() = (%#v, %t), want (%#v, true)", got, ok, req)
	}
}

func TestFromMDEmpty(t *testing.T) {
	req := FromMD(metadata.MD{})
	if req != (Request{}) {
		t.Fatalf("FromMD(empty) = %#v, want zero HeaderList", req)
	}
}

func TestFromContextWithoutMetadata(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("FromContext() ok = true on plain context, want false")
	}
}
