package kafka

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/observability/logger"
)

// logType is the LogPayload.Type value every log line this package emits
// carries, so messaging records are filterable alongside the storage and
// transport packages that follow the same convention.
const logType = "MESSAGING_KAFKA"

// newLogPayload builds the LogPayload skeleton shared by every log line in
// this package: correlation identifiers lifted out of ctx plus the
// operation's type/method/path. Call sites fill in Duration, Error and any
// Metadata they have.
func newLogPayload(ctx context.Context, appName, method, path string) logger.LogPayload {
	return logger.LogPayload{
		Timestamp: time.Now(),
		AppName:   appName,
		Type:      logType,
		Method:    method,
		TraceId:   tracing.TraceID(ctx),
		SessionId: tracing.SessionID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		Path:      path,
	}
}
