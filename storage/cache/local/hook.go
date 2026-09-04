package local

import (
	"context"
	"fmt"
	"strings"
	"time"

	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/observability/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Hook provides observability (tracing and logging) for local cache operations.
type Hook struct {
	logger  logger.Logger
	config  Config
	appName string
}

// NewHook creates a new Hook instance.
func NewHook(l logger.Logger, cfg Config, appName string) *Hook {
	return &Hook{
		logger:  l,
		config:  cfg,
		appName: appName,
	}
}

// Execute wraps an operation with tracing and logging.
func (h *Hook) Execute(ctx context.Context, op string, statement string, fn func() error) error {
	start := time.Now()

	var span trace.Span
	if h.config.Tracing.Enabled {
		tracer := otel.Tracer("local-cache")
		ctx, span = tracer.Start(ctx, fmt.Sprintf("local: %s", op),
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("db.system", "inmemory"),
				attribute.String("db.operation", op),
				attribute.String("db.statement", statement),
			),
		)
	}

	err := fn()

	if h.config.Tracing.Enabled {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}

	if h.config.Logger.Enabled && h.logger != nil {
		h.log(ctx, op, statement, start, err)
	}

	return err
}

func (h *Hook) log(ctx context.Context, op string, statement string, start time.Time, err error) {
	payload := logger.LogPayload{
		Timestamp: start,
		AppName:   h.appName,
		Type:      "STORAGE_LOCAL",
		Method:    strings.ToUpper(op),
		IpAddress: "in-memory",
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      op,
		Request:   statement,
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil {
		payload.Error = err.Error()
		h.logger.Error(ctx, fmt.Sprintf("LOCAL %s", op), payload)
	} else {
		h.logger.Info(ctx, fmt.Sprintf("LOCAL %s", op), payload)
	}
}
