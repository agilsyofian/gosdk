package metrics

// Config defines the configuration for Prometheus metrics.
type Config struct {
	Enabled   bool      `mapstructure:"enabled"`
	Namespace string    `mapstructure:"namespace"` // default: appName or "gosdk"
	Subsystem string    `mapstructure:"subsystem"`
	Buckets   []float64 `mapstructure:"buckets"` // latency histogram buckets in seconds
}

// DefaultBuckets provides standard HTTP/gRPC request duration buckets (in seconds).
// Covers high-resolution latencies from 5ms up to 10s.
var DefaultBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// DefaultConfig returns the default metrics configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:   true,
		Namespace: "gosdk",
		Buckets:   DefaultBuckets,
	}
}
