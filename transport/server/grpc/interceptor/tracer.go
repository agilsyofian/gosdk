package interceptor

import (
	"context"
	"encoding/hex"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
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
		md, _ := metadata.FromIncomingContext(ctx)
		if len(md) > 0 {
			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(md))
		}

		traceID := extractTraceID(ctx, md)
		sessionID := extractSessionID(ctx, md)

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

		syncMetadata(newCtx, traceID, sessionID, span)

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

func extractIncomingHeader(md metadata.MD, key string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

func extractTraceID(ctx context.Context, md metadata.MD) string {
	if id := extractIncomingHeader(md, MetadataTraceID); id != "" {
		return id
	}
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	if m, ok := coremetadata.FromContext(ctx); ok && m.TraceID != "" {
		return m.TraceID
	}
	u := uuid.New()
	return hex.EncodeToString(u[:])
}

func extractSessionID(ctx context.Context, md metadata.MD) string {
	if id := extractIncomingHeader(md, MetadataSessionID); id != "" {
		return id
	}
	if m, ok := coremetadata.FromContext(ctx); ok && m.SessionID != "" {
		return m.SessionID
	}
	u := uuid.New()
	return hex.EncodeToString(u[:])
}

func syncMetadata(ctx context.Context, traceID, sessionID string, span trace.Span) {
	m, ok := coremetadata.FromContext(ctx)
	if !ok {
		return
	}
	m.TraceID = traceID
	if sessionID != "" {
		m.SessionID = sessionID
	}

	if span.SpanContext().HasSpanID() {
		spanID := span.SpanContext().SpanID().String()
		if spanID != "0000000000000000" {
			m.SpanID = spanID
		}
	}
}
