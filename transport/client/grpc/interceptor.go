package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/core/resilience/circuitbreaker"
	"github.com/agilsyofian/gosdk/core/resilience/retry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MetadataTraceID and MetadataSessionID are the gRPC metadata keys
// injected by the Tracer interceptor. The canonical definitions live in
// core/metadata.
const (
	MetadataTraceID   = "x-trace-id"
	MetadataSessionID = "x-session-id"
)

// metadataCarrier is a custom carrier for gRPC metadata that ensures keys are lowercase.
type metadataCarrier metadata.MD

func (c metadataCarrier) Get(key string) string {
	values := metadata.MD(c).Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c metadataCarrier) Set(key string, value string) {
	metadata.MD(c).Set(key, value)
}

func (c metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// Retry interceptor
func Retry(p *retry.Policy) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return p.Execute(ctx, func(ctx context.Context) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}

// CircuitBreaker interceptor
func CircuitBreaker(p *circuitbreaker.Policy) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return p.Execute(ctx, func(ctx context.Context) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}

// Tracer interceptor
func Tracer(appName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		tracer := otel.Tracer("grpc-client")

		ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", appName, method),
			trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		// Inject trace_id and session_id from context to metadata
		if tid := coreContext.TraceID(ctx); tid != "" {
			md.Set(MetadataTraceID, tid)
		}
		if sid := coreContext.SessionID(ctx); sid != "" {
			md.Set(MetadataSessionID, sid)
		}

		otel.GetTextMapPropagator().Inject(ctx, metadataCarrier(md))
		ctx = metadata.NewOutgoingContext(ctx, md)

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			span.RecordError(err)
		}

		return err
	}
}

// Logger interceptor
func Logger(l logger.Logger, appName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()

		err := invoker(ctx, method, req, reply, cc, opts...)

		payload := logger.LogPayload{
			Timestamp: start,
			AppName:   appName,
			Type:      "INTEGRATION",
			Method:    method,
			TraceId:   coreContext.TraceID(ctx),
			SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
			SessionId: coreContext.SessionID(ctx),
			Path:      method,
			Request:   fmt.Sprintf("%+v", req),
			Duration:  time.Since(start).Milliseconds(),
		}

		if err != nil {
			payload.Error = err.Error()
			l.Error(ctx, fmt.Sprintf("GRPC CLIENT %s", method), payload)
			return err
		}

		payload.Response = fmt.Sprintf("%+v", reply)
		l.Info(ctx, fmt.Sprintf("GRPC CLIENT %s", method), payload)

		return nil
	}
}

// TokenProvider resolves a token for outgoing requests.
type TokenProvider func(ctx context.Context) (string, error)

// AuthorizationBearer interceptor injects Bearer token obtained from tokenProvider into outgoing gRPC metadata.
func AuthorizationBearer(tokenProvider TokenProvider) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		token, err := tokenProvider(ctx)
		if err != nil {
			return fmt.Errorf("authorization bearer: %w", err)
		}

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		md.Set("authorization", fmt.Sprintf("Bearer %s", token))
		ctx = metadata.NewOutgoingContext(ctx, md)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// PassThroughMetadata is an interceptor that passes through the outgoing
// gRPC metadata already present in ctx without modification. Use it when
// the caller is responsible for populating outgoing metadata (e.g. via
// metadata.NewOutgoingContext) before the RPC is invoked.
//
// To propagate HTTP request metadata (set by transport/server/http/middleware
// Metadata) into outgoing gRPC calls, convert it in the integration client
// using metadata.Pairs with the values from httpmetadata.FromContext(ctx)
// and attach it via metadata.NewOutgoingContext before calling the RPC.
func PassThroughMetadata() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
