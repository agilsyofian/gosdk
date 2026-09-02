package metadata

import (
	"context"
	"net/http"
	"testing"
)

func TestFromHeaderAndContext(t *testing.T) {
	header := http.Header{}
	header.Set(DeviceID, "device-123")
	header.Set(DeviceAppVersion, "1.2.3")
	header.Set(TraceID, "trace-123")

	request := FromHeader(header)
	if request.DeviceID != "device-123" || request.DeviceAppVersion != "1.2.3" || request.TraceID != "trace-123" {
		t.Fatalf("FromHeader() = %#v, want mapped header values", request)
	}

	ctx := WithContext(context.Background(), request)
	got, ok := FromContext(ctx)
	if !ok || got != request {
		t.Fatalf("FromContext() = (%#v, %t), want (%#v, true)", got, ok, request)
	}
}

func TestFromContextWithoutMetadata(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("FromContext() ok = true, want false")
	}
}
