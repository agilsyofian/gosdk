package tracing

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

// Tracing owns the OpenTelemetry tracer provider and its exporter.
type Tracing struct {
	tp *sdktrace.TracerProvider
}

// NewTracing is NewTracingContext with context.Background().
func NewTracing(cfg Config, appName, appVersion string) (*Tracing, error) {
	return NewTracingContext(context.Background(), cfg, appName, appVersion)
}

// NewTracingContext creates the OTLP/gRPC exporter and tracer provider and
// installs them, together with the W3C TraceContext and Baggage
// propagators, as the OpenTelemetry globals.
//
// When cfg.Enabled is false it returns a no-op Tracing and changes nothing.
// When the exporter cannot be created it returns an error and leaves the
// globals untouched, so a misconfigured collector fails at startup instead
// of silently dropping every span.
func NewTracingContext(ctx context.Context, cfg Config, appName, appVersion string) (*Tracing, error) {
	if !cfg.Enabled {
		return &Tracing{}, nil
	}

	exporterOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.TLS {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithTLSCredentials(
			credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}),
		))
	} else {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("tracing: create OTLP exporter for endpoint %q: %w", cfg.Endpoint, err)
	}

	res, errRes := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceName(appName),
			semconv.ServiceVersion(appVersion),
		),
	)
	if errRes != nil {
		// Release the exporter's gRPC connection; nothing else owns it yet.
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("tracing: create resource: %w", errRes)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SamplerRatio)),
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracing{tp: tp}, nil
}

// TracerProvider returns the underlying OpenTelemetry TracerProvider.
func (t *Tracing) TracerProvider() trace.TracerProvider {
	if t == nil || t.tp == nil {
		return nil
	}
	return t.tp
}

// Shutdown flushes buffered spans and closes the exporter, waiting at most
// five seconds. It is a no-op for a disabled Tracing.
func (t *Tracing) Shutdown(ctx context.Context) error {
	if t.tp == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return t.tp.Shutdown(ctx)
}
