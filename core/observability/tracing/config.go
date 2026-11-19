package tracing

// Config configures the OpenTelemetry tracer provider created by NewTracing.
type Config struct {
	// Enabled turns tracing on. When false, NewTracing returns a no-op Tracing.
	Enabled bool `mapstructure:"enabled"`
	// Endpoint is the OTLP/gRPC collector address, e.g. "otel-collector:4317".
	Endpoint string `mapstructure:"endpoint"`
	// SamplerRatio is the fraction of traces to sample (0.0–1.0).
	SamplerRatio float64 `mapstructure:"sampler_ratio"`
	// TLS sends spans to the collector over TLS, verified against the
	// system certificate pool. The default (false) keeps the plaintext
	// connection used by earlier versions.
	TLS bool `mapstructure:"use_tls"`
}
