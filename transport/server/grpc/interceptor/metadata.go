package interceptor

import (
	"context"
	"strings"

	"github.com/agilsyofian/gosdk/core/metadata"
	"google.golang.org/grpc"
	grpcMD "google.golang.org/grpc/metadata"
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
func Metadata(cfgs ...metadata.Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		incomingMD, _ := grpcMD.FromIncomingContext(ctx)
		m := metadata.Extract(mdCarrier(incomingMD), cfgs...)
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
		ctx = metadata.WithContext(ctx, m)
		return handler(srv, &wrappedStream{ss, ctx})
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
