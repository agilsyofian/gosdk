package interceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type CombinedRequest struct {
	Body string `json:"body"`
}

func Logger(l logger.Logger, appName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		md, _ := metadata.FromIncomingContext(ctx)

		resp, err := handler(ctx, req)

		logRPC(ctx, l, appName, rpcEvent{
			fullMethod: info.FullMethod,
			md:         md,
			req:        req,
			resp:       resp,
			err:        err,
			start:      start,
		})

		return resp, err
	}
}

// LoggerStream returns a stream server interceptor that logs the lifecycle, status, and duration of streaming RPCs.
func LoggerStream(l logger.Logger, appName string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)

		err := handler(srv, ss)

		logRPC(ctx, l, appName, rpcEvent{
			fullMethod: info.FullMethod,
			md:         md,
			req:        nil,
			resp:       nil,
			err:        err,
			start:      start,
		})

		return err
	}
}

type rpcEvent struct {
	fullMethod string
	md         metadata.MD
	req        any
	resp       any
	err        error
	start      time.Time
}

func logRPC(
	ctx context.Context,
	l logger.Logger,
	appName string,
	ev rpcEvent,
) {
	statusCode := int(status.Code(ev.err))
	payload := logger.LogPayload{
		Timestamp: ev.start,
		AppName:   appName,
		Type:      "TRANSPORT",
		Method:    "GRPC",
		IpAddress: resolveClientIP(ctx),
		Status:    statusCode,
		Metadata:  ev.md,
		TraceId:   coreContext.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: coreContext.SessionID(ctx),
		Path:      ev.fullMethod,
		Request:   formatRequest(ev.req),
		Response:  formatResponse(ev.resp, ev.err),
		Duration:  time.Since(ev.start).Milliseconds(),
	}

	logFunc := determineLogFunc(l, ev.err)
	if ev.err != nil {
		payload.Error = ev.err.Error()
	}

	logFunc(
		ctx,
		fmt.Sprintf("GRPC %s", ev.fullMethod),
		payload,
	)
}

func formatRequest(req any) any {
	if req == nil {
		return nil
	}
	bodyStr := ""
	if protoStr := marshalProto(req); protoStr != nil {
		if s, ok := protoStr.(string); ok {
			bodyStr = s
		} else if b, err := json.Marshal(protoStr); err == nil {
			bodyStr = string(b)
		} else {
			bodyStr = fmt.Sprint(protoStr)
		}
	}
	fullRequest, err := json.Marshal(CombinedRequest{
		Body: bodyStr,
	})
	if err != nil {
		return bodyStr
	}
	return string(fullRequest)
}

func formatResponse(resp any, err error) any {
	if resp != nil {
		return marshalProto(resp)
	}
	if err != nil {
		st, _ := status.FromError(err)
		errResp := map[string]any{
			"code":    st.Code().String(),
			"message": st.Message(),
		}
		if details := st.Details(); len(details) > 0 {
			errResp["details"] = details
		}
		b, err := json.Marshal(errResp)
		if err != nil {
			return nil
		}
		return string(b)
	}
	return nil
}

func determineLogFunc(l logger.Logger, err error) func(context.Context, string, logger.LogPayload) {
	if err == nil {
		return l.Info
	}
	switch status.Code(err) {
	case codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.ResourceExhausted,
		codes.FailedPrecondition:
		return l.Warn
	default:
		return l.Error
	}
}

func resolveClientIP(ctx context.Context) string {
	if m, ok := coremetadata.FromContext(ctx); ok && m.ClientIP != "" {
		return m.ClientIP
	}
	return extractClientIP(ctx, "")
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
