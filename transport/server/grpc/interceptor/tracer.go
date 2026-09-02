package interceptor

import (
	"context"
	"encoding/hex"

	coreContext "github.com/agilsyofian/gosdk/observability/tracing"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	MetadataTraceID   = "x-trace-id"
	MetadataSessionID = "x-session-id"
)

func Tracer(appName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(md))
		}

		traceID, sessionID := extractIdentifiers(ctx)

		tracer := otel.GetTracerProvider().Tracer(appName)
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithAttributes(
				attribute.String("rpc.system", "grpc"),
				attribute.String("rpc.method", info.FullMethod),
				attribute.String("rpc.session_id", sessionID),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		newCtx := coreContext.WithAppName(ctx, appName)
		newCtx = coreContext.WithTraceID(newCtx, traceID)
		newCtx = coreContext.WithSessionID(newCtx, sessionID)

		header := metadata.Pairs(
			MetadataTraceID, traceID,
			MetadataSessionID, sessionID,
		)

		otel.GetTextMapPropagator().Inject(newCtx, &metadataTextMapCarrier{MD: header})

		grpc.SetHeader(ctx, header)

		return handler(newCtx, req)
	}
}

type metadataTextMapCarrier struct {
	metadata.MD
}

func (c *metadataTextMapCarrier) Get(key string) string {
	values := c.MD.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c *metadataTextMapCarrier) Set(key string, value string) {
	c.MD.Set(key, value)
}

func (c *metadataTextMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c.MD))
	for k := range c.MD {
		keys = append(keys, k)
	}
	return keys
}

func extractIdentifiers(ctx context.Context) (string, string) {
	var traceID, sessionID string

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(MetadataTraceID); len(values) > 0 {
			traceID = values[0]
		}

		if traceID == "" {
			span := trace.SpanFromContext(ctx)
			if span.SpanContext().HasTraceID() {
				traceID = span.SpanContext().TraceID().String()
			}
		}

		if values := md.Get(MetadataSessionID); len(values) > 0 {
			sessionID = values[0]
		}
	}

	if traceID == "" {
		u := uuid.New()
		traceID = hex.EncodeToString(u[:])
	}
	if sessionID == "" {
		u := uuid.New()
		sessionID = hex.EncodeToString(u[:])
	}

	return traceID, sessionID
}
