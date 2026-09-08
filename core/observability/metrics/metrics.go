package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the registered Prometheus metrics and custom registry.
type Metrics struct {
	cfg Config
	reg *prometheus.Registry

	// HTTP Server metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight *prometheus.GaugeVec

	// gRPC Server metrics
	grpcRequestsTotal    *prometheus.CounterVec
	grpcRequestDuration  *prometheus.HistogramVec
	grpcRequestsInFlight *prometheus.GaugeVec
}

// New creates and initializes a Metrics registry with standard RED metrics and Go process metrics.
func New(cfg Config) *Metrics {
	if len(cfg.Buckets) == 0 {
		cfg.Buckets = DefaultBuckets
	}

	reg := prometheus.NewRegistry()

	// Register standard Go runtime & process collectors
	reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)

	m := &Metrics{
		cfg: cfg,
		reg: reg,

		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests processed, partitioned by status code, method, and path.",
			},
			[]string{"method", "path", "status"},
		),

		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "http_request_duration_seconds",
				Help:      "Histogram of HTTP request latencies in seconds, partitioned by status code, method, and path.",
				Buckets:   cfg.Buckets,
			},
			[]string{"method", "path", "status"},
		),

		httpRequestsInFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "http_requests_in_flight",
				Help:      "Current number of HTTP requests being processed.",
			},
			[]string{"method"},
		),

		grpcRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "grpc_server_requests_total",
				Help:      "Total number of gRPC requests processed, partitioned by service, method, and status code.",
			},
			[]string{"service", "method", "code"},
		),

		grpcRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "grpc_server_request_duration_seconds",
				Help:      "Histogram of gRPC request latencies in seconds, partitioned by service, method, and status code.",
				Buckets:   cfg.Buckets,
			},
			[]string{"service", "method", "code"},
		),

		grpcRequestsInFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "grpc_server_requests_in_flight",
				Help:      "Current number of gRPC requests being processed.",
			},
			[]string{"service"},
		),
	}

	reg.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.httpRequestsInFlight,
		m.grpcRequestsTotal,
		m.grpcRequestDuration,
		m.grpcRequestsInFlight,
	)

	return m
}

// Registry returns the underlying prometheus.Registry.
func (m *Metrics) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}
	return m.reg
}

// HTTPHandler returns an http.Handler that serves the Prometheus scrape endpoint.
func (m *Metrics) HTTPHandler() http.Handler {
	if m == nil || m.reg == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordHTTPRequest records duration, status code, method, and path for an HTTP request.
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	if m == nil || !m.cfg.Enabled {
		return
	}
	statusStr := strconv.Itoa(status)
	m.httpRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.httpRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
}

// IncHTTPInFlight increments the in-flight gauge for the given method.
func (m *Metrics) IncHTTPInFlight(method string) {
	if m == nil || !m.cfg.Enabled {
		return
	}
	m.httpRequestsInFlight.WithLabelValues(method).Inc()
}

// DecHTTPInFlight decrements the in-flight gauge for the given method.
func (m *Metrics) DecHTTPInFlight(method string) {
	if m == nil || !m.cfg.Enabled {
		return
	}
	m.httpRequestsInFlight.WithLabelValues(method).Dec()
}

// RecordGRPCRequest records duration, status code, service, and method for a gRPC request.
func (m *Metrics) RecordGRPCRequest(service, method, code string, duration time.Duration) {
	if m == nil || !m.cfg.Enabled {
		return
	}
	m.grpcRequestsTotal.WithLabelValues(service, method, code).Inc()
	m.grpcRequestDuration.WithLabelValues(service, method, code).Observe(duration.Seconds())
}

// IncGRPCInFlight increments the in-flight gauge for the given service.
func (m *Metrics) IncGRPCInFlight(service string) {
	if m == nil || !m.cfg.Enabled {
		return
	}
	m.grpcRequestsInFlight.WithLabelValues(service).Inc()
}

// DecGRPCInFlight decrements the in-flight gauge for the given service.
func (m *Metrics) DecGRPCInFlight(service string) {
	if m == nil || !m.cfg.Enabled {
		return
	}
	m.grpcRequestsInFlight.WithLabelValues(service).Dec()
}
