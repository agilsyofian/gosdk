package http

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func Tracer(appName string) Middleware {
	return func(next RoundTripper) RoundTripper {
		return func(ctx context.Context, req *http.Request) (*http.Response, error) {
			tracer := otel.Tracer("http-client")

			ctx, span := tracer.Start(ctx, appName+" "+req.Method+" "+req.URL.Path,
				trace.WithSpanKind(trace.SpanKindClient))
			defer span.End()

			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

			req = req.WithContext(ctx)

			resp, err := next(ctx, req)

			if err != nil {
				span.RecordError(err)
			}

			return resp, err
		}
	}
}
