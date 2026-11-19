package tracing

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace/noop"
)

// sentinelPropagator is a comparable (pointer) propagator used to detect
// whether NewTracing replaced the global one.
type sentinelPropagator struct{ propagation.TraceContext }

// preserveGlobals snapshots the OpenTelemetry globals NewTracing mutates and
// installs known sentinels, restoring the originals when the test ends.
func preserveGlobals(t *testing.T) (noop.TracerProvider, *sentinelPropagator) {
	t.Helper()
	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})
	sentinelTP := noop.NewTracerProvider()
	sentinelProp := &sentinelPropagator{}
	otel.SetTracerProvider(sentinelTP)
	otel.SetTextMapPropagator(sentinelProp)
	return sentinelTP, sentinelProp
}

func shutdown(t *testing.T, tr *Tracing) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tr.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestNewTracing_Disabled(t *testing.T) {
	sentinelTP, _ := preserveGlobals(t)

	tr, err := NewTracing(Config{Enabled: false, Endpoint: "otel-collector:4317\n"}, "svc", "v1")
	if err != nil {
		t.Fatalf("NewTracing() error = %v, want nil when disabled", err)
	}
	if tr == nil || tr.TracerProvider() != nil {
		t.Fatalf("NewTracing() = %#v, want no-op Tracing without a provider", tr)
	}
	if otel.GetTracerProvider() != sentinelTP {
		t.Error("disabled tracing must not replace the global TracerProvider")
	}
	shutdown(t, tr)
}

func TestNewTracing_InvalidEndpointReturnsError(t *testing.T) {
	// Endpoints that make the OTLP exporter fail, e.g. a value read from a
	// secret or env file with a trailing newline. Previously this error was
	// swallowed and tracing was silently disabled.
	for _, endpoint := range []string{"otel-collector:4317\n", "%zz"} {
		t.Run(endpoint, func(t *testing.T) {
			sentinelTP, sentinelProp := preserveGlobals(t)

			tr, err := NewTracing(Config{Enabled: true, Endpoint: endpoint, SamplerRatio: 1}, "svc", "v1")
			if err == nil {
				shutdown(t, tr)
				t.Fatal("NewTracing() error = nil, want exporter creation error")
			}
			if tr != nil {
				t.Errorf("NewTracing() returned %#v alongside an error, want nil", tr)
			}
			if !strings.Contains(err.Error(), "create OTLP exporter") {
				t.Errorf("error %q does not identify the failing step", err)
			}
			if otel.GetTracerProvider() != sentinelTP {
				t.Error("failed NewTracing must not replace the global TracerProvider")
			}
			if p, ok := otel.GetTextMapPropagator().(*sentinelPropagator); !ok || p != sentinelProp {
				t.Error("failed NewTracing must not replace the global TextMapPropagator")
			}
		})
	}
}

func TestNewTracing_EnabledInstallsGlobals(t *testing.T) {
	for _, tc := range []struct {
		name string
		tls  bool
	}{
		{name: "plaintext", tls: false},
		{name: "tls", tls: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preserveGlobals(t)

			// The gRPC connection is lazy, so no collector needs to be running.
			tr, err := NewTracingContext(context.Background(), Config{
				Enabled:      true,
				Endpoint:     "localhost:4317",
				SamplerRatio: 1,
				TLS:          tc.tls,
			}, "svc", "v1")
			if err != nil {
				t.Fatalf("NewTracingContext() error = %v", err)
			}
			defer shutdown(t, tr)

			if tr.TracerProvider() == nil {
				t.Fatal("TracerProvider() = nil, want the SDK provider")
			}
			if otel.GetTracerProvider() != tr.TracerProvider() {
				t.Error("global TracerProvider was not set to the new provider")
			}
			fields := strings.Join(otel.GetTextMapPropagator().Fields(), ",")
			for _, want := range []string{"traceparent", "baggage"} {
				if !strings.Contains(fields, want) {
					t.Errorf("global propagator fields %q missing %q", fields, want)
				}
			}
		})
	}
}
