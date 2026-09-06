package interceptor

import (
	"context"
	"encoding/hex"
	"net"
	"strings"

	"github.com/agilsyofian/gosdk/core/metadata"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	grpcMD "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type mdCarrier grpcMD.MD

func (m mdCarrier) Get(key string) string {
	vals := grpcMD.MD(m).Get(strings.ToLower(key))
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (m mdCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Metadata extracts the standard incoming request metadata from gRPC MD
// and stores it in context as *core/metadata.Metadata.
// If client IP or tracing IDs are missing, it falls back to peer connection IP and auto-generated IDs.
func Metadata(cfgs ...metadata.Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		incomingMD, _ := grpcMD.FromIncomingContext(ctx)
		m := metadata.Extract(mdCarrier(incomingMD), cfgs...)

		m.ClientIP = extractClientIP(ctx, m.ClientIP)
		ensureTraceAndSpanIDs(m)

		ctx = metadata.WithContext(ctx, m)
		return handler(ctx, req)
	}
}

// MetadataStream is the stream variant of Metadata.
func MetadataStream(cfgs ...metadata.Config) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		incomingMD, _ := grpcMD.FromIncomingContext(ctx)
		m := metadata.Extract(mdCarrier(incomingMD), cfgs...)

		m.ClientIP = extractClientIP(ctx, m.ClientIP)
		ensureTraceAndSpanIDs(m)

		ctx = metadata.WithContext(ctx, m)
		return handler(srv, &wrappedStream{ss, ctx})
	}
}

func extractClientIP(ctx context.Context, headerIP string) string {
	if headerIP != "" {
		if idx := strings.Index(headerIP, ","); idx != -1 {
			return strings.TrimSpace(headerIP[:idx])
		}
		return strings.TrimSpace(headerIP)
	}

	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return "Unknown"
	}

	addr := p.Addr.String()
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return ip
}

func ensureTraceAndSpanIDs(m *metadata.Metadata) {
	if m == nil {
		return
	}
	if m.TraceID == "" {
		u := uuid.New()
		m.TraceID = hex.EncodeToString(u[:])
	}
	if m.SpanID == "" {
		u := uuid.New()
		m.SpanID = hex.EncodeToString(u[:8])
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
