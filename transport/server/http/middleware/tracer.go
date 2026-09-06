package middleware

import (
	"context"
	"encoding/hex"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	MetadataTraceID   = "x-trace-id"
	MetadataSessionID = "x-session-id"
)

func Tracer(appName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		traceID := extractTraceID(c, span)
		sessionID := extractSessionID(c)

		span.SetAttributes(
			attribute.String("http.session_id", sessionID),
		)

		c.Writer.Header().Set(MetadataTraceID, traceID)
		c.Writer.Header().Set(MetadataSessionID, sessionID)

		ctx := c.Request.Context()
		ctx = tracing.WithAppName(ctx, appName)
		ctx = tracing.WithTraceID(ctx, traceID)
		ctx = tracing.WithSessionID(ctx, sessionID)

		syncMetadata(ctx, traceID, sessionID, span)

		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(c.Writer.Header()))

		c.Request = c.Request.WithContext(ctx)

		c.Next()

		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
		)
	}
}

func extractTraceID(c *gin.Context, span trace.Span) string {
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	if headerID := c.GetHeader(MetadataTraceID); headerID != "" {
		return headerID
	}
	if m, ok := coremetadata.FromContext(c.Request.Context()); ok && m.TraceID != "" {
		return m.TraceID
	}
	u := uuid.New()
	return hex.EncodeToString(u[:])
}

func extractSessionID(c *gin.Context) string {
	if headerID := c.GetHeader(MetadataSessionID); headerID != "" {
		return headerID
	}
	if m, ok := coremetadata.FromContext(c.Request.Context()); ok && m.SessionID != "" {
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
	m.SessionID = sessionID

	if span.SpanContext().HasSpanID() {
		spanID := span.SpanContext().SpanID().String()
		if spanID != "0000000000000000" {
			m.SpanID = spanID
		}
	}
}
