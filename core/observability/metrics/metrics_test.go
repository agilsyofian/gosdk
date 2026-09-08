package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_RecordHTTPRequest_And_HTTPHandler(t *testing.T) {
	cfg := Config{
		Enabled:   true,
		Namespace: "test_app",
		Subsystem: "http",
	}

	m := New(cfg)
	require.NotNil(t, m)

	m.IncHTTPInFlight("GET")
	m.RecordHTTPRequest("GET", "/example/v1/auth/password", 200, 150*time.Millisecond)
	m.RecordHTTPRequest("POST", "/example/v1/auth/password", 500, 250*time.Millisecond)
	m.DecHTTPInFlight("GET")

	m.RecordGRPCRequest("user.ServiceUser", "GetUser", "0", 50*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/actuator/prometheus", nil)
	rec := httptest.NewRecorder()

	handler := m.HTTPHandler()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Verify custom HTTP metrics exist in Prometheus output
	assert.Contains(t, body, "test_app_http_http_requests_total")
	assert.Contains(t, body, `path="/example/v1/auth/password"`)
	assert.Contains(t, body, `status="200"`)
	assert.Contains(t, body, `status="500"`)
	assert.Contains(t, body, "test_app_http_http_request_duration_seconds")

	// Verify gRPC metrics
	assert.Contains(t, body, "test_app_http_grpc_server_requests_total")

	// Verify standard Go process metrics are included
	assert.True(t, strings.Contains(body, "go_goroutines") || strings.Contains(body, "process_cpu_seconds_total"))
}
