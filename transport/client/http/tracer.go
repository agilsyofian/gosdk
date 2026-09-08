package http

import (
	"context"
	"net/http"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	MetadataTraceID   = "x-trace-id"
	MetadataSessionID = "x-session-id"
)

func Tracer(appName string) Middleware {
	return func(next RoundTripper) RoundTripper {
		return func(ctx context.Context, req *http.Request) (*http.Response, error) {
			tracer := otel.Tracer("http-client")

			ctx, span := tracer.Start(ctx, appName+" "+req.Method+" "+req.URL.Path,
				trace.WithSpanKind(trace.SpanKindClient))
			defer span.End()

			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

			injectTraceID(ctx, span, req.Header)
			injectSessionID(ctx, req.Header)

			req = req.WithContext(ctx)

			resp, err := next(ctx, req)

			if err != nil {
				span.RecordError(err)
			}

			return resp, err
		}
	}
}

func injectTraceID(ctx context.Context, span trace.Span, header http.Header) {
	if header.Get(MetadataTraceID) != "" {
		return
	}
	if tid := coreContext.TraceID(ctx); tid != "" {
		header.Set(MetadataTraceID, tid)
		return
	}
	if m, ok := coremetadata.FromContext(ctx); ok && m.TraceID != "" {
		header.Set(MetadataTraceID, m.TraceID)
		return
	}
	if span.SpanContext().HasTraceID() {
		header.Set(MetadataTraceID, span.SpanContext().TraceID().String())
	}
}

func injectSessionID(ctx context.Context, header http.Header) {
	if header.Get(MetadataSessionID) != "" {
		return
	}
	if sid := coreContext.SessionID(ctx); sid != "" {
		header.Set(MetadataSessionID, sid)
		return
	}
	if m, ok := coremetadata.FromContext(ctx); ok && m.SessionID != "" {
		header.Set(MetadataSessionID, m.SessionID)
	}
}
