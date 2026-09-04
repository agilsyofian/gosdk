package interceptor

import (
	"context"

	grpcmetadata "github.com/agilsyofian/gosdk/transport/server/grpc/metadata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Metadata extracts the SDK's standard incoming request metadata from the
// gRPC MD and stores it in the context, where handlers and services can
// retrieve it via metadata.FromContext(ctx).
//
// It mirrors the behaviour of transport/server/http/middleware.Metadata,
// providing a consistent API across both transports.
func Metadata() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = grpcmetadata.WithContext(ctx, grpcmetadata.FromMD(md))
		return handler(ctx, req)
	}
}

// MetadataStream is the stream variant of Metadata.
func MetadataStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		ctx = grpcmetadata.WithContext(ctx, grpcmetadata.FromMD(md))
		return handler(srv, &wrappedStream{ss, ctx})
	}
}

// wrappedStream substitutes the context on an existing ServerStream.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
