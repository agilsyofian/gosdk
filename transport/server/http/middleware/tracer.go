package middleware

import (
	"encoding/hex"

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
		traceID := ""
		if span.SpanContext().HasTraceID() {
			traceID = span.SpanContext().TraceID().String()
		}

		if traceID == "" {
			traceID = c.GetHeader(MetadataTraceID)
		}

		if traceID == "" {
			u := uuid.New()
			traceID = hex.EncodeToString(u[:])
		}

		sessionID := c.GetHeader(MetadataSessionID)
		if sessionID == "" {
			u := uuid.New()
			sessionID = hex.EncodeToString(u[:])
		}

		span.SetAttributes(
			attribute.String("http.session_id", sessionID),
		)

		c.Writer.Header().Set(MetadataTraceID, traceID)
		c.Writer.Header().Set(MetadataSessionID, sessionID)

		ctx := c.Request.Context()
		ctx = tracing.WithAppName(ctx, appName)
		ctx = tracing.WithTraceID(ctx, traceID)
		ctx = tracing.WithSessionID(ctx, sessionID)

		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(c.Writer.Header()))

		c.Request = c.Request.WithContext(ctx)

		c.Next()

		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
		)
	}
}
