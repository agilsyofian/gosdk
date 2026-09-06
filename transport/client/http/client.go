package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/resilience/circuitbreaker"
	"github.com/agilsyofian/gosdk/resilience/retry"
)

type IClient interface {
	Execute(ctx context.Context, method, path string, body any, dest any, opts ...RequestOption) error
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	logger     logger.Logger
	config     Config
	middleware []Middleware
}

type RequestOption func(*http.Request)

type Middleware func(next RoundTripper) RoundTripper

type RoundTripper func(ctx context.Context, req *http.Request) (*http.Response, error)

func New(baseURL string, timeout time.Duration, l logger.Logger, cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
		logger:  l,
		config:  cfg,
	}
}

func (c *Client) Use(m ...Middleware) {
	c.middleware = append(c.middleware, m...)
}

func (c *Client) Execute(ctx context.Context, method, path string, body any, dest any, opts ...RequestOption) error {
	return c.Do(ctx, method, path, body, dest, opts...)
}

func (c *Client) Do(ctx context.Context, method, path string, body any, dest any, opts ...RequestOption) error {
	identifier := c.config.ServerName + path
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s: marshal body: %w", identifier, err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("%s: new request: %w", identifier, err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, opt := range opts {
		opt(req)
	}

	operation := c.operationFactory()

	resp, err := operation(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: read response body: %w", identifier, err)
	}

	if dest != nil && len(bodyBytes) > 0 {
		if errX := json.Unmarshal(bodyBytes, dest); errX != nil {
			return &ServerError{
				Code:  resp.StatusCode,
				Path:  identifier,
				Cause: errX,
				Body:  bodyBytes,
			}
		}
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return &ServerError{
			Code: resp.StatusCode,
			Path: identifier,
			Body: bodyBytes,
		}
	}

	return nil
}

func (c *Client) operationFactory() func(ctx context.Context, req *http.Request) (*http.Response, error) {
	operation := func(ctx context.Context, req *http.Request) (*http.Response, error) {
		return c.httpClient.Do(req)
	}

	if c.logger != nil {
		operation = Logger(c.logger, c.config.ServerName)(operation)
	}

	for i := len(c.middleware) - 1; i >= 0; i-- {
		operation = c.middleware[i](operation)
	}

	if c.config.Retry.Enabled {
		operation = Retry(retry.New(c.config.Retry))(operation)
	}

	if c.config.CircuitBreaker.Enabled {
		operation = CircuitBreaker(circuitbreaker.New(c.config.CircuitBreaker))(operation)
	}

	operation = Tracer(c.config.ServerName)(operation)
	return operation
}

func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set(key, value)
	}
}
