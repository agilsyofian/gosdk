package interceptor

import (
	"context"

	goError "errors"

	"github.com/agilsyofian/gosdk/core/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Recovery(appName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				var appErr error
				switch x := r.(type) {
				case error:
					appErr = errors.InternalServerError(x, appName, errors.StatusInternalServerError.String(), "An error occurred")
				default:
					appErr = errors.Unknown(goError.New("unknown error"))
				}

				// Map to gRPC status
				err = status.Errorf(codes.Internal, "%v", appErr)
			}
		}()

		return handler(ctx, req)
	}
}
