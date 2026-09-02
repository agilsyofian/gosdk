package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	coreContext "github.com/agilsyofian/gosdk/observability/tracing"
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
	filterParams []string
	maxLength    int
}

// NewZerologLogger creates a new ZerologLogger instance.
func NewZerologLogger(config Config) Logger {

	// Global zerolog configuration for standardized field names
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFieldName = "time"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "msg"

	var output io.Writer
	switch config.Output {
	case "", "stdout":
		output = os.Stdout

	case "stderr":
		output = os.Stderr

	case "file":
		file, err := os.OpenFile(config.LogDir, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		output = file

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
	z.applyAppFields(event, payload)
}

func (z *ZerologLogger) applyTracingFields(ctx context.Context, event *zerolog.Event, payload LogPayload) {
	if payload.TraceId != "" {
		event.Str("trace_id", payload.TraceId)
	} else if traceID := coreContext.TraceID(ctx); traceID != "" {
		event.Str("trace_id", traceID)
	}

	if payload.SessionId != "" {
		event.Str("session_id", payload.SessionId)
	} else if requestID := coreContext.SessionID(ctx); requestID != "" {
		event.Str("session_id", requestID)
	}

	if payload.SpanId != "" && payload.SpanId != "0000000000000000" {
		event.Str("span_id", payload.SpanId)
	} else if trace.SpanContextFromContext(ctx).SpanID().String() != "0000000000000000" {
		event.Str("span_id", trace.SpanContextFromContext(ctx).SpanID().String())
	}
}

func (z *ZerologLogger) applyTransportFields(event *zerolog.Event, payload LogPayload) {
	if payload.IpAddress != "" {
		event.Str("ip_address", payload.IpAddress)
	}
	if payload.Type != "" {
		event.Str("type", payload.Type)
	}
	if payload.Method != "" {
		event.Str("method", payload.Method)
	}
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

func (z *ZerologLogger) applyAppFields(event *zerolog.Event, payload LogPayload) {
	if payload.AppName != "" {
		event.Str("app_name", payload.AppName)
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

func formatValue(val any, filterParams []string, maxLength int) any {
	switch sv := val.(type) {
	case string:
		if len(sv) > maxLength {
			return sv[:maxLength] + "..."
		}
		return sv
	case map[string]any, []any:
		processValue(sv, filterParams, maxLength)
		return sv
	default:
		return val
	}
}

func shouldFilter(key string, filterParams []string) bool {
	for _, p := range filterParams {
		if strings.EqualFold(key, p) {
			return true
		}
	}
	return false
}
