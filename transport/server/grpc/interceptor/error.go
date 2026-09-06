package interceptor

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreErrors "github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Error(l logger.Logger, appName string, showErrorDetail bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		finalErr := logAndTransformError(ctx, l, appName, info.FullMethod, err, showErrorDetail)
		return nil, finalErr
	}
}

// ErrorStream returns a stream server interceptor that catches, logs, and transforms stream errors.
func ErrorStream(l logger.Logger, appName string, showErrorDetail bool) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		if err == nil {
			return nil
		}

		return logAndTransformError(ss.Context(), l, appName, info.FullMethod, err, showErrorDetail)
	}
}

func logAndTransformError(
	ctx context.Context,
	l logger.Logger,
	appName, fullMethod string,
	err error,
	showErrorDetail bool,
) error {
	finalErr, statusCode := handleInterceptorError(err, showErrorDetail)

	payload := logger.LogPayload{
		Timestamp: time.Now(),
		AppName:   appName,
		Type:      "ERROR",
		Method:    "GRPC",
		IpAddress: resolveClientIP(ctx),
		Status:    int(statusCode),
		TraceId:   tracing.TraceID(ctx),
		SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
		SessionId: tracing.SessionID(ctx),
		Path:      fullMethod,
		Error:     finalErr.Error(),
	}

	l.Error(
		ctx,
		fmt.Sprintf("ERROR %s", appName),
		payload,
	)

	return finalErr
}

func handleInterceptorError(err error, showErrorDetail bool) (error, codes.Code) {
	var appErr *coreErrors.AppError
	if errors.As(err, &appErr) {
		code := mapHTTPStatusToGRPCCode(appErr.Status.Int())
		msg := appErr.Message
		if showErrorDetail && appErr.Error() != "" {
			msg = fmt.Sprintf("%s: %s", appErr.Message, appErr.Error())
		}
		return status.Error(code, msg), code
	}

	if st, ok := status.FromError(err); ok {
		return st.Err(), st.Code()
	}

	if showErrorDetail {
		return status.Errorf(codes.Internal, "%v", err), codes.Internal
	}
	return status.Error(codes.Internal, "Internal server error"), codes.Internal
}

func mapHTTPStatusToGRPCCode(httpStatus int) codes.Code {
	switch httpStatus {
	case 400:
		return codes.InvalidArgument
	case 401:
		return codes.Unauthenticated
	case 403:
		return codes.PermissionDenied
	case 404:
		return codes.NotFound
	case 429:
		return codes.ResourceExhausted
	default:
		return codes.Internal
	}
}
