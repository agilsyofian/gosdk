package interceptor

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/metadata"
	"google.golang.org/grpc"
	grpcMD "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type mockAddr struct {
	addr string
}

func (m mockAddr) Network() string { return "tcp" }
func (m mockAddr) String() string  { return m.addr }

func TestMetadata_PeerIPExtraction(t *testing.T) {
	interceptor := Metadata()

	// Simulate gRPC call with peer context and no proxy headers
	p := &peer.Peer{
		Addr: mockAddr{addr: "192.168.1.100:54321"},
	}
	ctx := peer.NewContext(context.Background(), p)

	var extractedMeta *metadata.Metadata
	handler := func(ctx context.Context, req any) (any, error) {
		m, ok := metadata.FromContext(ctx)
		if !ok {
			t.Fatal("expected metadata in context")
		}
		extractedMeta = m
		return struct{}{}, nil
	}

	_, err := interceptor(ctx, struct{}{}, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if extractedMeta.ClientIP != "192.168.1.100" {
		t.Fatalf("expected ClientIP '192.168.1.100', got: '%s'", extractedMeta.ClientIP)
	}
	if extractedMeta.TraceID == "" {
		t.Fatal("expected non-empty generated TraceID")
	}
	if extractedMeta.SpanID == "" {
		t.Fatal("expected non-empty generated SpanID")
	}
}

func TestMetadata_HeaderIPOverride(t *testing.T) {
	interceptor := Metadata()

	// Both peer context and x-forwarded-for header present
	p := &peer.Peer{
		Addr: mockAddr{addr: "10.0.0.1:12345"},
	}
	ctx := peer.NewContext(context.Background(), p)
	md := grpcMD.Pairs("x-forwarded-for", "203.0.113.195, 70.41.3.18")
	ctx = grpcMD.NewIncomingContext(ctx, md)

	var extractedMeta *metadata.Metadata
	handler := func(ctx context.Context, req any) (any, error) {
		m, ok := metadata.FromContext(ctx)
		if !ok {
			t.Fatal("expected metadata in context")
		}
		extractedMeta = m
		return struct{}{}, nil
	}

	_, err := interceptor(ctx, struct{}{}, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should prioritize the client IP from x-forwarded-for (first entry)
	if extractedMeta.ClientIP != "203.0.113.195" {
		t.Fatalf("expected ClientIP '203.0.113.195', got: '%s'", extractedMeta.ClientIP)
	}
}

func TestMetadata_UnknownFallback(t *testing.T) {
	interceptor := Metadata()

	// Context with neither peer nor headers
	var extractedMeta *metadata.Metadata
	handler := func(ctx context.Context, req any) (any, error) {
		m, ok := metadata.FromContext(ctx)
		if !ok {
			t.Fatal("expected metadata in context")
		}
		extractedMeta = m
		return struct{}{}, nil
	}

	_, err := interceptor(context.Background(), struct{}{}, &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if extractedMeta.ClientIP != "Unknown" {
		t.Fatalf("expected ClientIP 'Unknown', got: '%s'", extractedMeta.ClientIP)
	}
	if extractedMeta.TraceID == "" {
		t.Fatal("expected non-empty TraceID")
	}
}

func TestMetadataStream_Parity(t *testing.T) {
	streamInterceptor := MetadataStream()

	p := &peer.Peer{
		Addr: mockAddr{addr: "172.16.0.50:60000"},
	}
	ctx := peer.NewContext(context.Background(), p)
	md := grpcMD.Pairs("x-trace-id", "custom-trace-123", "x-span-id", "custom-span-456")
	ctx = grpcMD.NewIncomingContext(ctx, md)

	ss := &mockServerStream{ctx: ctx}

	var extractedMeta *metadata.Metadata
	handler := func(srv any, stream grpc.ServerStream) error {
		m, ok := metadata.FromContext(stream.Context())
		if !ok {
			t.Fatal("expected metadata in stream context")
		}
		extractedMeta = m
		return nil
	}

	err := streamInterceptor(struct{}{}, ss, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if extractedMeta.ClientIP != "172.16.0.50" {
		t.Fatalf("expected ClientIP '172.16.0.50', got: '%s'", extractedMeta.ClientIP)
	}
	if extractedMeta.TraceID != "custom-trace-123" {
		t.Fatalf("expected TraceID 'custom-trace-123', got: '%s'", extractedMeta.TraceID)
	}
	if extractedMeta.SpanID != "custom-span-456" {
		t.Fatalf("expected SpanID 'custom-span-456', got: '%s'", extractedMeta.SpanID)
	}
}
