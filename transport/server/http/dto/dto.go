package dto

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"go.opentelemetry.io/otel/trace"
)

type BaseResponse struct {
	Source    string `json:"source" omitzero:"true"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Reference string `json:"reference"`
	Error     string `json:"error,omitempty" omitzero:"true"`
	Result    any    `json:"result,omitempty"`
}

func New(ctx context.Context, result any) BaseResponse {
	traceID := tracing.TraceID(ctx)
	// Fallback to OTEL trace ID if context trace ID is empty
	if traceID == "" {
		if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
			traceID = span.SpanContext().TraceID().String()
		}
	}

	return BaseResponse{
		Source:    tracing.AppName(ctx),
		Reference: traceID,
		Result:    result,
	}
}

func (b BaseResponse) WithSource(source string) BaseResponse {
	b.Source = source
	return b
}

func (b BaseResponse) WithCode(code string) BaseResponse {
	b.Code = code
	return b
}

func (b BaseResponse) WithMessage(message string) BaseResponse {
	b.Message = message
	return b
}

func (b BaseResponse) WithError(err string) BaseResponse {
	b.Error = err
	return b
}

func (b BaseResponse) Success() BaseResponse {
	b.Code = "00"
	b.Message = "Success"
	return b
}
