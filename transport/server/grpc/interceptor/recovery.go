package interceptor

import (
	"context"
	goError "errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/core/errors"
	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// SourceLocation contains stack trace location details where a panic occurred.
type SourceLocation struct {
	File     string `json:"file,omitempty"`
	Function string `json:"function,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// Recovery returns a unary server interceptor that catches panics, extracts
// stack trace information, logs a structured error, and returns an internal gRPC status.
func Recovery(l logger.Logger, appName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = handlePanic(ctx, r, info.FullMethod, l, appName)
			}
		}()

		return handler(ctx, req)
	}
}

// RecoveryStream returns a stream server interceptor that catches panics in stream RPCs.
func RecoveryStream(l logger.Logger, appName string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = handlePanic(ss.Context(), r, info.FullMethod, l, appName)
			}
		}()

		return handler(srv, ss)
	}
}

func handlePanic(ctx context.Context, r any, method string, l logger.Logger, appName string) error {
	loc, stackTrace := parseStackTrace(debug.Stack())

	var appErr error
	switch x := r.(type) {
	case error:
		appErr = errors.InternalServerError(x, appName, errors.StatusInternalServerError.String(), "An error occurred")
	default:
		appErr = errors.Unknown(goError.New("unknown error"))
	}

	if l != nil {
		md, _ := metadata.FromIncomingContext(ctx)
		clientIP := ""
		if m, ok := coremetadata.FromContext(ctx); ok {
			clientIP = m.ClientIP
		}

		l.Error(ctx, fmt.Sprintf("panic recovered in gRPC %s", method), logger.LogPayload{
			Timestamp: time.Now(),
			AppName:   appName,
			Type:      "ERROR",
			Method:    "GRPC",
			IpAddress: clientIP,
			Metadata:  md,
			TraceId:   tracing.TraceID(ctx),
			SpanId:    trace.SpanContextFromContext(ctx).SpanID().String(),
			SessionId: tracing.SessionID(ctx),
			Path:      method,
			Status:    int(codes.Internal),
			Error: map[string]any{
				"panic":    fmt.Sprintf("%v", r),
				"file":     loc.File,
				"function": loc.Function,
				"line":     loc.Line,
				"stack":    stackTrace,
			},
		})
	}

	return status.Errorf(codes.Internal, "%v", appErr)
}

func parseStackTrace(stack []byte) (SourceLocation, string) {
	stackStr := string(stack)
	lines := strings.Split(stackStr, "\n")
	return findSourceLocation(lines), stackStr
}

func findSourceLocation(lines []string) SourceLocation {
	for i := 1; i < len(lines)-1; i++ {
		line := strings.TrimSpace(lines[i])
		if isRuntimeOrFrameworkFrame(line) {
			continue
		}
		if i+1 < len(lines) && strings.Contains(lines[i+1], ".go:") {
			return extractLocationFromFrame(line, lines[i+1])
		}
	}
	return SourceLocation{}
}

func isRuntimeOrFrameworkFrame(line string) bool {
	return strings.HasPrefix(line, "goroutine ") ||
		strings.Contains(line, "runtime/debug.") ||
		strings.Contains(line, "runtime/panic.") ||
		strings.Contains(line, "interceptor.handlePanic") ||
		strings.Contains(line, "interceptor.Recovery") ||
		strings.Contains(line, "middleware.Recovery")
}

func extractLocationFromFrame(fnLine, fileLine string) SourceLocation {
	loc := SourceLocation{Function: fnLine}
	cleaned := strings.TrimSpace(fileLine)
	if idx := strings.LastIndex(cleaned, " +0x"); idx != -1 {
		cleaned = cleaned[:idx]
	}
	parts := strings.Split(cleaned, ":")
	if len(parts) >= 2 {
		loc.File = parts[0]
		if l, err := strconv.Atoi(parts[1]); err == nil {
			loc.Line = l
		}
	}
	return loc
}
