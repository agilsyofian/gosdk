package logger

import (
	"context"
	"time"
)

// LogPayload defines the standardized structure for log data.
type LogPayload struct {
	Timestamp time.Time           `json:"timestamp" omitzero:"true"`
	AppName   string              `json:"appName,omitempty"`
	Type      string              `json:"type" omitzero:"true"`
	Method    string              `json:"method,omitempty"`
	IpAddress string              `json:"ipAddress,omitempty"`
	TraceId   string              `json:"traceId,omitempty"`
	SessionId string              `json:"sessionId,omitempty"`
	SpanId    string              `json:"spanId,omitempty"`
	Path      string              `json:"path,omitempty"`
	Status    int                 `json:"status,omitempty"`
	Metadata  map[string][]string `json:"metadata,omitempty"`
	Request   any                 `json:"request,omitempty"`
	Response  any                 `json:"response,omitempty"`
	Duration  int64               `json:"duration,omitempty"`
	Error     any                 `json:"error,omitempty"`
}

// WithMetadata adds metadata to the payload and returns it.
func (l LogPayload) WithMetadata(key string, values ...string) LogPayload {
	if l.Metadata == nil {
		l.Metadata = make(map[string][]string)
	}
	l.Metadata[key] = append(l.Metadata[key], values...)
	return l
}

// WithError sets the error in the payload and returns it.
func (l LogPayload) WithError(err any) LogPayload {
	l.Error = err
	return l
}

// Logger is a structured logging interface used throughout this framework.
// Any implementation (slog, zap, zerolog, logrus) can be adapted to this interface.
type Logger interface {
	Debug(ctx context.Context, msg string, payload LogPayload)
	Info(ctx context.Context, msg string, payload LogPayload)
	Warn(ctx context.Context, msg string, payload LogPayload)
	Error(ctx context.Context, msg string, payload LogPayload)
}

// nopLogger discards every record.
type nopLogger struct{}

func (nopLogger) Debug(context.Context, string, LogPayload) { return }
func (nopLogger) Info(context.Context, string, LogPayload)  { return }
func (nopLogger) Warn(context.Context, string, LogPayload)  { return }
func (nopLogger) Error(context.Context, string, LogPayload) { return }

// Nop returns a Logger that discards everything written to it. It is the
// safe default for packages that accept an injected Logger, so they never
// have to nil-check before logging.
func Nop() Logger { return nopLogger{} }
