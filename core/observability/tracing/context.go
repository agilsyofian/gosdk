package tracing

import (
	"context"
)

type contextKey string

const (
	appNameKey   contextKey = "app_name"
	traceIDKey   contextKey = "trace_id"   // Long-lived session
	sessionIDKey contextKey = "session_id" // Single lifecycle
)

// Metadata retrieved from context or headers.
type Metadata map[string]string

// WithAppName inserts App FullName into the context.
func WithAppName(ctx context.Context, appName string) context.Context {
	return context.WithValue(ctx, appNameKey, appName)
}

// AppName takes App FullName from the context.
func AppName(ctx context.Context) string {
	if id, ok := ctx.Value(appNameKey).(string); ok {
		return id
	}
	return "server"
}

// WithTraceID inserts a trace ID (long-lived session) into the context.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// TraceID takes the trace ID from the context.
func TraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

// WithSessionID inserts the session ID (single lifecycle) into the context.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// SessionID retrieves the session ID from the context.
func SessionID(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey).(string); ok {
		return id
	}
	return ""
}
