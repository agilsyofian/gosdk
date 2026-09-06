package grpc

import (
	"context"
	"fmt"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/resilience/circuitbreaker"
	"github.com/agilsyofian/gosdk/core/resilience/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn         *grpc.ClientConn
	logger       logger.Logger
	config       Config
	interceptors []grpc.UnaryClientInterceptor
}

func New(l logger.Logger, cfg Config) *Client {
	return &Client{
		logger: l,
		config: cfg,
	}
}

func (c *Client) Use(interceptors ...grpc.UnaryClientInterceptor) {
	c.interceptors = append(c.interceptors, interceptors...)
}

func (c *Client) Dial(ctx context.Context, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	unaryInterceptors := []grpc.UnaryClientInterceptor{
		Tracer(c.config.ServerName),
		CircuitBreaker(circuitbreaker.New(c.config.CircuitBreaker)),
		Retry(retry.New(c.config.Retry)),
	}

	if c.logger != nil {
		unaryInterceptors = append(unaryInterceptors, Logger(c.logger, c.config.ServerName))
	}

	// User defined interceptors are added last (inner-most in gRPC chain order is reverse of append usually,
	// but gRPC ChainUnaryClientInterceptor applies them in order: first is outer-most).
	// For HTTP client, user middlewares were applied before resilience.
	// In gRPC, ChainUnaryClientInterceptor(a, b, c) -> a(b(c(handler))).
	// So a is outer-most.

	finalInterceptors := append(unaryInterceptors, c.interceptors...)

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(finalInterceptors...),
	}

	dialOpts = append(dialOpts, opts...)

	conn, err := grpc.NewClient(c.config.Addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", c.config.Addr, err)
	}

	c.conn = conn
	return conn, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Conn() *grpc.ClientConn {
	return c.conn
}
