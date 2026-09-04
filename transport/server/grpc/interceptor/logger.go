package interceptor

import (
	"context"
	"fmt"
	"time"

	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/observability/logger"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func Logger(l logger.Logger, appName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		md, _ := metadata.FromIncomingContext(ctx)

		resp, err := handler(ctx, req)

		payload := logger.LogPayload{
			Timestamp: start,
			AppName:   appName,
			Type:      "TRANSPORT",
			Method:    "GRPC",
			Metadata:  md,
			TraceId:   coreContext.TraceID(ctx),
			SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
			SessionId: coreContext.SessionID(ctx),
			Path:      info.FullMethod,
			Request:   marshalProto(req),
			Response:  marshalProto(resp),
			Duration:  time.Since(start).Milliseconds(),
		}

		l.Info(
			ctx,
			fmt.Sprintf("GRPC %s", info.FullMethod),
			payload,
		)

		return resp, err
	}
}

func marshalProto(m any) any {
	if m == nil {
		return nil
	}
	if msg, ok := m.(proto.Message); ok {
		opts := protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: true,
		}
		b, err := opts.Marshal(msg)
		if err != nil {
			return m
		}
		return string(b)
	}
	return m
}
