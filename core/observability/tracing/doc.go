// Package tracing sets up OpenTelemetry distributed tracing for gosdk services.
//
// NewTracing (or NewTracingContext) builds an OTLP/gRPC exporter and an SDK
// tracer provider from Config and installs them, with the W3C TraceContext
// and Baggage propagators, as the OpenTelemetry globals. Call Shutdown on
// exit to flush buffered spans.
//
// With Config.Enabled false the constructors return a no-op Tracing and leave
// the globals untouched. Exporter or resource errors are returned rather than
// silently disabling tracing, so a misconfigured collector fails at startup.
//
// The package also provides context helpers (TraceID and friends) used by
// the logger and transport packages to correlate logs with traces.
package tracing
