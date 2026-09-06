package redis

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Hook struct {
	logger  logger.Logger
	config  Config
	appName string
}

func NewHook(l logger.Logger, cfg Config, appName string) *Hook {
	return &Hook{
		logger:  l,
		config:  cfg,
		appName: appName,
	}
}

func (h *Hook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *Hook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()

		var span trace.Span
		if h.config.Tracing.Enabled {
			tracer := otel.Tracer("redis-client")
			ctx, span = tracer.Start(ctx, fmt.Sprintf("redis: %s", cmd.FullName()),
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					attribute.String("db.system", "redis"),
					attribute.String("db.operation", cmd.FullName()),
					attribute.String("db.statement", cmd.String()),
				),
			)
			// defer span.End() is removed, we will call it manually to capture correct timing/error
		}

		err := next(ctx, cmd)

		if h.config.Tracing.Enabled {
			if err != nil && err != redis.Nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()
		}

		if h.config.Logger.Enabled {
			h.log(ctx, cmd, start, err)
		}

		return err
	}
}

func (h *Hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()

		var span trace.Span
		if h.config.Tracing.Enabled {
			tracer := otel.Tracer("redis-client")
			ctx, span = tracer.Start(ctx, "redis: pipeline",
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					attribute.String("db.system", "redis"),
					attribute.Int("db.redis.pipeline_size", len(cmds)),
				),
			)
			// defer span.End() is removed
		}

		err := next(ctx, cmds)

		if h.config.Tracing.Enabled {
			if err != nil && err != redis.Nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()
		}

		if h.config.Logger.Enabled {
			// For pipeline, we log a summary
			var cmdNames []string
			for _, cmd := range cmds {
				cmdNames = append(cmdNames, cmd.FullName())
			}
			h.logPipeline(ctx, cmdNames, start, err)
		}

		return err
	}
}

func (h *Hook) log(ctx context.Context, cmd redis.Cmder, start time.Time, err error) {
	payload := logger.LogPayload{
		Timestamp: start,
		AppName:   h.appName,
		Type:      "STORAGE_REDIS",
		Method:    strings.ToUpper(cmd.FullName()),
		IpAddress: fmt.Sprintf("%v", h.config.Addrs),
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      cmd.FullName(),
		Request:   cmd.String(),
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil && err != redis.Nil {
		payload.Error = err.Error()
		h.logger.Error(ctx, fmt.Sprintf("REDIS %s", cmd.FullName()), payload)
	} else {
		if err == redis.Nil {
			payload.Response = "null"
		}
		h.logger.Info(ctx, fmt.Sprintf("REDIS %s", cmd.FullName()), payload)
	}
}

func (h *Hook) logPipeline(ctx context.Context, cmdNames []string, start time.Time, err error) {
	summary := strings.Join(cmdNames, ", ")
	payload := logger.LogPayload{
		Timestamp: start,
		AppName:   h.appName,
		Type:      "STORAGE_REDIS",
		Method:    "PIPELINE",
		IpAddress: fmt.Sprintf("%v", h.config.Addrs),
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      "pipeline",
		Request:   summary,
		Duration:  time.Since(start).Milliseconds(),
	}

	if err != nil && err != redis.Nil {
		payload.Error = err.Error()
		h.logger.Error(ctx, "REDIS PIPELINE", payload)
	} else {
		h.logger.Info(ctx, "REDIS PIPELINE", payload)
	}
}
