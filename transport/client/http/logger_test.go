package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/stretchr/testify/assert"
)

type mockClientLogger struct {
	lastLevel   string
	lastMsg     string
	lastPayload logger.LogPayload
}

func (m *mockClientLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "debug"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *mockClientLogger) Info(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "info"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *mockClientLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "warn"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *mockClientLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "error"
	m.lastMsg = msg
	m.lastPayload = payload
}

func TestHTTPClientLogger_Success(t *testing.T) {
	mockL := &mockClientLogger{}
	middleware := Logger(mockL, "client-app")

	handler := middleware(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"status":"ok"}`)),
		}, nil
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://api.example.com/items?page=1", bytes.NewBufferString(`{"name":"test"}`))
	resp, err := handler(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "info", mockL.lastLevel)
	assert.Equal(t, http.StatusOK, mockL.lastPayload.Status)
	assert.Equal(t, "INTEGRATION", mockL.lastPayload.Type)
	assert.Equal(t, http.MethodPost, mockL.lastPayload.Method)
	assert.Contains(t, mockL.lastPayload.Request.(string), `"body":"{\"name\":\"test\"}"`)
	assert.Contains(t, mockL.lastPayload.Request.(string), `"query":{"page":"1"}`)
	assert.Equal(t, `{"status":"ok"}`, mockL.lastPayload.Response)
}

func TestHTTPClientLogger_Unauthorized_Warn(t *testing.T) {
	mockL := &mockClientLogger{}
	middleware := Logger(mockL, "client-app")

	handler := middleware(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"unauthorized"}`)),
		}, nil
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://api.example.com/protected", nil)
	resp, err := handler(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "warn", mockL.lastLevel)
	assert.Equal(t, http.StatusUnauthorized, mockL.lastPayload.Status)
	assert.Equal(t, `{"error":"unauthorized"}`, mockL.lastPayload.Response)
}

func TestHTTPClientLogger_NetworkError(t *testing.T) {
	mockL := &mockClientLogger{}
	middleware := Logger(mockL, "client-app")

	networkErr := errors.New("connection refused")
	handler := middleware(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return nil, networkErr
	})

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://api.example.com/down", nil)
	resp, err := handler(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "error", mockL.lastLevel)
	assert.Equal(t, "connection refused", mockL.lastPayload.Error)
	assert.Contains(t, mockL.lastPayload.Response.(string), `"code":"CLIENT_ERROR"`)
	assert.Contains(t, mockL.lastPayload.Response.(string), `"message":"connection refused"`)
}
