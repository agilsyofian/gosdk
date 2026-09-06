package interceptor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/transport/idempotency"
	"google.golang.org/grpc"
	gmetadata "google.golang.org/grpc/metadata"
)

type dummyResponse struct {
	Message string
}

func TestIdempotencyInterceptor_MetadataKey(t *testing.T) {
	storage := idempotency.NewMemoryStorage()
	interceptor := Idempotency(storage,
		WithTTL(time.Minute),
		WithResponseFactory(func(method string) any {
			return &dummyResponse{}
		}),
	)

	var calls int32
	handler := func(ctx context.Context, req any) (any, error) {
		atomic.AddInt32(&calls, 1)
		return &dummyResponse{Message: "hello"}, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/CreateOrder"}
	ctx := gmetadata.NewIncomingContext(context.Background(), gmetadata.Pairs("x-idempotency-key", "key-123"))

	// 1st invocation
	resp1, err := interceptor(ctx, "req1", info, handler)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if resp1.(*dummyResponse).Message != "hello" {
		t.Fatalf("unexpected message: %v", resp1)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	// 2nd invocation (same key)
	resp2, err := interceptor(ctx, "req1", info, handler)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if resp2 == nil {
		t.Fatalf("expected non-nil response on replay")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected calls to remain 1, got %d", calls)
	}
}

func TestIdempotencyInterceptor_CustomKeyFunc(t *testing.T) {
	storage := idempotency.NewMemoryStorage()
	interceptor := Idempotency(storage,
		WithCanonicalKeyFunc(func(ctx context.Context, req any, info *grpc.UnaryServerInfo) (string, error) {
			return "custom:" + req.(string), nil
		}),
		WithResponseFactory(func(method string) any {
			return &dummyResponse{}
		}),
	)

	var calls int32
	handler := func(ctx context.Context, req any) (any, error) {
		atomic.AddInt32(&calls, 1)
		return &dummyResponse{Message: "custom-ok"}, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Submit"}
	ctx := context.Background()

	_, err := interceptor(ctx, "user-456", info, handler)
	if err != nil {
		t.Fatalf("call 1 failed: %v", err)
	}

	_, err = interceptor(ctx, "user-456", info, handler)
	if err != nil {
		t.Fatalf("call 2 failed: %v", err)
	}

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestIdempotencyInterceptor_IgnoreMethods(t *testing.T) {
	storage := idempotency.NewMemoryStorage()
	interceptor := Idempotency(storage,
		WithIgnoreMethods("/test.Service/GetOrder"),
	)

	var calls int32
	handler := func(ctx context.Context, req any) (any, error) {
		atomic.AddInt32(&calls, 1)
		return &dummyResponse{Message: "order"}, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/GetOrder"}
	ctx := gmetadata.NewIncomingContext(context.Background(), gmetadata.Pairs("x-idempotency-key", "ignored-key"))

	_, _ = interceptor(ctx, "req", info, handler)
	_, _ = interceptor(ctx, "req", info, handler)

	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected ignored method to execute 2 times, got %d", calls)
	}
}

func TestIdempotencyInterceptor_NoKeyPassThrough(t *testing.T) {
	storage := idempotency.NewMemoryStorage()
	interceptor := Idempotency(storage)

	var calls int32
	handler := func(ctx context.Context, req any) (any, error) {
		atomic.AddInt32(&calls, 1)
		return &dummyResponse{Message: "pass"}, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/NoKey"}
	ctx := context.Background() // No metadata

	_, _ = interceptor(ctx, "non-proto-req", info, handler)
	_, _ = interceptor(ctx, "non-proto-req", info, handler)

	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls without key, got %d", calls)
	}
}
