package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

const (
	maxJSONSize         = 1024 * 1024 // 1MB
	filteredText        = "****"
	largeFieldMessage   = "field %s is too large"
	defaultMaxLogLength = 256
)

var _ Logger = (*ZerologLogger)(nil)

// ZerologLogger is an implementation of Logger using rs/zerolog.
type ZerologLogger struct {
	logger       zerolog.Logger
	appName      string
	filterParams []string
	maxLength    int
}

func init() {
	// Global zerolog configuration for standardized field names
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFieldName = "time"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "msg"
}

// NewZerologLogger creates a new ZerologLogger instance.
func NewZerologLogger(config Config) Logger {
	var output io.Writer
	switch config.Output {
	case "", "stdout":
		output = os.Stdout

	case "stderr":
		output = os.Stderr

	case "file":
		output = openLogFile(config.LogDir)

	case "both":
		output = io.MultiWriter(os.Stdout, openLogFile(config.LogDir))

	default:
		panic(fmt.Sprintf("invalid log output: %s", config.Output))
	}

	l := zerolog.New(output).
		Level(config.LogLevel).
		With().
		Timestamp().
		Logger()

	return &ZerologLogger{
		logger:       l,
		appName:      config.AppName,
		filterParams: config.FilterParams,
		maxLength:    config.MaxLengthParamValue,
	}
}

// Debug logs a message at Debug level.
func (z *ZerologLogger) Debug(ctx context.Context, msg string, payload LogPayload) {
	z.log(ctx, z.logger.Debug(), msg, payload)
}

// Info logs a message at Info level.
func (z *ZerologLogger) Info(ctx context.Context, msg string, payload LogPayload) {
	z.log(ctx, z.logger.Info(), msg, payload)
}

// Warn logs a message at Warn level.
func (z *ZerologLogger) Warn(ctx context.Context, msg string, payload LogPayload) {
	z.log(ctx, z.logger.Warn(), msg, payload)
}

// Error logs a message at Error level.
func (z *ZerologLogger) Error(ctx context.Context, msg string, payload LogPayload) {
	z.log(ctx, z.logger.Error(), msg, payload)
}

func (z *ZerologLogger) log(ctx context.Context, event *zerolog.Event, msg string, payload LogPayload) {
	if event == nil {
		return
	}

	z.applyFields(ctx, event, payload)

	event.Msg(msg)
}

func (z *ZerologLogger) applyFields(ctx context.Context, event *zerolog.Event, payload LogPayload) {
	z.applyTracingFields(ctx, event, payload)
	z.applyTransportFields(event, payload)
	z.applyDataFields(event, payload)
	z.applyAppFields(ctx, event, payload)
}

func (z *ZerologLogger) applyTracingFields(ctx context.Context, event *zerolog.Event, payload LogPayload) {
	if tid := resolveTraceID(ctx, payload.TraceId); tid != "" {
		event.Str("trace_id", tid)
	}

	if sid := resolveSessionID(ctx, payload.SessionId); sid != "" {
		event.Str("session_id", sid)
	}

	if spid := resolveSpanID(ctx, payload.SpanId); spid != "" {
		event.Str("span_id", spid)
	}
}

func resolveTraceID(ctx context.Context, initial string) string {
	if initial != "" || ctx == nil {
		return initial
	}
	if tid := tracing.TraceID(ctx); tid != "" {
		return tid
	}
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return traceID(ctx)
}

func resolveSessionID(ctx context.Context, initial string) string {
	if initial != "" || ctx == nil {
		return initial
	}
	if sid := tracing.SessionID(ctx); sid != "" {
		return sid
	}
	return sessionID(ctx)
}

func resolveSpanID(ctx context.Context, initial string) string {
	if initial != "" && initial != "0000000000000000" {
		return initial
	}
	if ctx == nil {
		return ""
	}
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasSpanID() {
		sid := span.SpanContext().SpanID().String()
		if sid != "0000000000000000" {
			return sid
		}
	}
	return ""
}

func (z *ZerologLogger) applyTransportFields(event *zerolog.Event, payload LogPayload) {
	if payload.IpAddress != "" {
		event.Str("ip_address", payload.IpAddress)
	}
	typeVal := payload.Type
	if typeVal == "" {
		typeVal = "CONSOLE"
	}
	event.Str("type", typeVal)

	methodVal := payload.Method
	if methodVal == "" {
		methodVal = "CONSOLE"
	}
	event.Str("method", methodVal)
	if payload.Path != "" {
		event.Str("path", payload.Path)
	}
	if payload.Status != 0 {
		event.Int("status", payload.Status)
	}
	if payload.Duration != 0 {
		event.Int64("duration_ms", payload.Duration)
	}
}

func (z *ZerologLogger) applyDataFields(event *zerolog.Event, payload LogPayload) {
	if len(payload.Metadata) > 0 {
		meta := FilterSensitiveData(payload.Metadata, z.filterParams, z.maxLength)
		event.Interface("metadata", meta)
	}
	if payload.Request != nil {
		req := FilterSensitiveData(payload.Request, z.filterParams, z.maxLength)
		event.Interface("request", req)
	}
	if payload.Response != nil {
		res := FilterSensitiveData(payload.Response, z.filterParams, z.maxLength)
		event.Interface("response", res)
	}
	if payload.Error != nil {
		event.Interface("error", payload.Error)
	}
}

func (z *ZerologLogger) applyAppFields(ctx context.Context, event *zerolog.Event, payload LogPayload) {
	if z.appName != "" {
		event.Str("app_name", z.appName)
		return
	}
	if payload.AppName != "" {
		event.Str("app_name", payload.AppName)
		return
	}
	if app := appName(ctx); app != "" {
		event.Str("app_name", app)
	}
}

// FilterSensitiveData processes data to filter out sensitive information and handle oversized fields.
func FilterSensitiveData(data any, filterParams []string, maxLength int) map[string]any {
	if data == nil {
		return nil
	}

	m := convertToMap(data)
	if m == nil {
		return nil
	}

	if val, ok := m["error"]; ok && strings.Contains(fmt.Sprint(val), "too large") {
		return m
	}

	effectiveMaxLength := maxLength
	if effectiveMaxLength <= 0 {
		effectiveMaxLength = defaultMaxLogLength
	}

	processValue(m, filterParams, effectiveMaxLength)

	return m
}

func convertToMap(data any) map[string]any {
	switch v := data.(type) {
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, val := range v {
			m[k] = val
		}
		return m
	case []byte:
		return unmarshalJSON(v)
	case string:
		return unmarshalJSON([]byte(v))
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return unmarshalJSON(b)
	}
}

func unmarshalJSON(b []byte) map[string]any {
	if len(b) > maxJSONSize {
		return map[string]any{"error": fmt.Sprintf(largeFieldMessage, "body")}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"raw": string(b)}
	}
	return m
}

func processValue(data any, filterParams []string, maxLength int) {
	switch v := data.(type) {
	case map[string]any:
		processMap(v, filterParams, maxLength)
	case []any:
		processSlice(v, filterParams, maxLength)
	}
}

func processMap(m map[string]any, filterParams []string, maxLength int) {
	for key, val := range m {
		if shouldFilter(key, filterParams) {
			switch val.(type) {
			case map[string]any, []any:
				// Jika object/array, telusuri isinya
				processValue(val, filterParams, maxLength)
			default:
				// Jika primitif, masking
				m[key] = filteredText
			}
			continue
		}
		m[key] = formatValue(val, filterParams, maxLength)
	}
}

func processSlice(s []any, filterParams []string, maxLength int) {
	for i, val := range s {
		s[i] = formatValue(val, filterParams, maxLength)
	}
}

func openLogFile(logDir string) *os.File {
	if dir := filepath.Dir(logDir); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	file, err := os.OpenFile(logDir, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	return file
}

func formatValue(val any, filterParams []string, maxLength int) any {
	switch sv := val.(type) {
	case string:
		return formatStringValue(sv, filterParams, maxLength)
	case map[string]any, []any:
		processValue(sv, filterParams, maxLength)
		return sv
	default:
		return val
	}
}

func formatStringValue(s string, filterParams []string, maxLength int) string {
	trimmed := strings.TrimSpace(s)
	if isJSONString(trimmed) {
		var nested any
		if err := json.Unmarshal([]byte(trimmed), &nested); err == nil {
			processValue(nested, filterParams, maxLength)
			if b, err := json.Marshal(nested); err == nil {
				return string(b)
			}
		}
	}
	if len(s) > maxLength {
		return s[:maxLength] + "..."
	}
	return s
}

func isJSONString(s string) bool {
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func shouldFilter(key string, filterParams []string) bool {
	for _, p := range filterParams {
		if strings.EqualFold(key, p) {
			return true
		}
	}
	return false
}

type contextKey string

const (
	appNameKey   contextKey = "app_name"
	traceIDKey   contextKey = "trace_id"   // Long-lived session
	sessionIDKey contextKey = "session_id" // Single lifecycle
)

func appName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if name, ok := ctx.Value(appNameKey).(string); ok {
		return name
	}
	return ""
}

// WithAppName returns a copy of parent context in which app_name is set.
func WithAppName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, appNameKey, name)
}

func traceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

func sessionID(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey).(string); ok {
		return id
	}
	return ""
}
