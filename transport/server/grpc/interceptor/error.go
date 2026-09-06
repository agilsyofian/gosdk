package interceptor

import (
	"context"
	"errors"
	"fmt"
	"time"

	coreContext "github.com/agilsyofian/gosdk/core/errors"
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

		payload := logger.LogPayload{
			Timestamp: time.Now(),
			AppName:   appName,
			Type:      "ERROR",
			Method:    "GRPC",
			TraceId:   tracing.TraceID(ctx),
			SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
			SessionId: tracing.SessionID(ctx),
			Path:      info.FullMethod,
			Error:     err.Error(),
		}

		defer func() {
			l.Error(
				ctx,
				fmt.Sprintf("ERROR %s", appName),
				payload,
			)
		}()

		var appErr *coreContext.AppError
		if errors.As(err, &appErr) {
			payload.Error = appErr.Error()

			// Map AppError status to gRPC codes
			var code codes.Code
			switch appErr.Status.Int() {
			case 400:
				code = codes.InvalidArgument
			case 401:
				code = codes.Unauthenticated
			case 403:
				code = codes.PermissionDenied
			case 404:
				code = codes.NotFound
			case 429:
				code = codes.ResourceExhausted
			default:
				code = codes.Internal
			}

			msg := appErr.Message
			if showErrorDetail && appErr.Error() != "" {
				msg = fmt.Sprintf("%s: %s", appErr.Message, appErr.Error())
			}

			st := status.New(code, msg)
			// You could potentially add more detail to gRPC status here using st.WithDetails

			return nil, st.Err()
		}

		if showErrorDetail {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		return nil, status.Error(codes.Internal, "Internal server ExpectedError")
	}
}
