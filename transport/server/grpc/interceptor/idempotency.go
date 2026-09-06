package interceptor

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/transport/idempotency"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	gmetadata "google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	// ErrMissingGRPCIdempotencyKey is returned when a required idempotency key is missing.
	ErrMissingGRPCIdempotencyKey = errors.New("idempotency key is required")
)

// IdempotencyStorage is an alias to standard transport/idempotency.Storage.
type IdempotencyStorage = idempotency.Storage

// CanonicalKeyFunc defines the signature for extracting a canonical key from gRPC requests.
type CanonicalKeyFunc func(ctx context.Context, req any, info *grpc.UnaryServerInfo) (string, error)

// ResponseFactory creates an empty response message instance for a given gRPC method.
type ResponseFactory func(method string) any

// IdempotencyConfig configures the gRPC idempotency interceptor.
type IdempotencyConfig struct {
	Storage         IdempotencyStorage
	KeyFunc         CanonicalKeyFunc
	TTL             time.Duration
	LockTimeout     time.Duration
	MetadataKey     string
	IgnoreMethods   []string
	ResponseFactory ResponseFactory
}

// IdempotencyOption provides functional options to configure the gRPC Idempotency interceptor.
type IdempotencyOption func(*IdempotencyConfig)

// WithTTL sets custom TTL for cached idempotent responses (default: 30s).
func WithTTL(ttl time.Duration) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if ttl > 0 {
			c.TTL = ttl
		}
	}
}

// WithLockTimeout sets maximum wait time for concurrent in-flight requests (default: 10s).
func WithLockTimeout(timeout time.Duration) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if timeout > 0 {
			c.LockTimeout = timeout
		}
	}
}

// WithMetadataKey sets the incoming gRPC metadata key to inspect (default: "x-idempotency-key").
func WithMetadataKey(key string) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if key != "" {
			c.MetadataKey = strings.ToLower(key)
		}
	}
}

// WithCanonicalKeyFunc sets custom canonical key generation logic.
func WithCanonicalKeyFunc(fn CanonicalKeyFunc) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		if fn != nil {
			c.KeyFunc = fn
		}
	}
}

// WithIgnoreMethods specifies gRPC full method names to ignore.
func WithIgnoreMethods(methods ...string) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		c.IgnoreMethods = methods
	}
}

// WithResponseFactory supplies a factory to construct responses for unmarshaling during replay.
func WithResponseFactory(factory ResponseFactory) IdempotencyOption {
	return func(c *IdempotencyConfig) {
		c.ResponseFactory = factory
	}
}

// Idempotency returns a gRPC UnaryServerInterceptor that enforces idempotent execution.
func Idempotency(storage IdempotencyStorage, opts ...IdempotencyOption) grpc.UnaryServerInterceptor {
	cfg := IdempotencyConfig{
		Storage:       storage,
		TTL:           30 * time.Second,
		LockTimeout:   10 * time.Second,
		MetadataKey:   "x-idempotency-key",
		IgnoreMethods: []string{},
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.Storage == nil {
		cfg.Storage = idempotency.NewMemoryStorage()
	}

	engine := idempotency.NewEngine(cfg.Storage,
		idempotency.WithDefaultTTL(cfg.TTL),
		idempotency.WithLockTimeout(cfg.LockTimeout),
	)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if shouldIgnoreGRPCMethod(info.FullMethod, cfg.IgnoreMethods) {
			return handler(ctx, req)
		}

		key, err := resolveGRPCKey(ctx, req, info, cfg)
		if err != nil || key == "" {
			// If no idempotency key is present, pass through to normal execution
			return handler(ctx, req)
		}

		storageKey := "GRPC:IDEM:" + key

		var freshResp any
		rec, isReplay, execErr := engine.Execute(ctx, storageKey, cfg.TTL, func(execCtx context.Context) (*idempotency.Record, error) {
			resp, hErr := handler(execCtx, req)
			if hErr != nil {
				return nil, hErr
			}
			freshResp = resp
			return recordFromResponse(resp)
		})

		if execErr != nil {
			return nil, handleGRPCExecutionError(execErr)
		}

		if isReplay && rec != nil {
			return replayGRPCResponse(ctx, info.FullMethod, rec, cfg)
		}

		return freshResp, nil
	}
}

func shouldIgnoreGRPCMethod(method string, ignored []string) bool {
	for _, m := range ignored {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

func resolveGRPCKey(ctx context.Context, req any, info *grpc.UnaryServerInfo, cfg IdempotencyConfig) (string, error) {
	if cfg.KeyFunc != nil {
		return cfg.KeyFunc(ctx, req, info)
	}

	// Default: check incoming gRPC metadata
	if md, ok := gmetadata.FromIncomingContext(ctx); ok {
		vals := md.Get(cfg.MetadataKey)
		if len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			return strings.TrimSpace(vals[0]), nil
		}
	}

	// Fallback: SHA256 of Method + Serialized Request Body
	if pm, ok := req.(proto.Message); ok {
		reqBytes, err := proto.Marshal(pm)
		if err == nil {
			return idempotency.HashString(info.FullMethod + ":" + idempotency.HashBytes(reqBytes)), nil
		}
	}

	return "", nil
}

func recordFromResponse(resp any) (*idempotency.Record, error) {
	headers := make(map[string]string)
	var body []byte

	if pm, ok := resp.(proto.Message); ok {
		protoFullName := string(pm.ProtoReflect().Descriptor().FullName())
		headers["proto_type"] = protoFullName

		b, err := proto.Marshal(pm)
		if err != nil {
			return nil, err
		}
		body = b
	}

	return &idempotency.Record{
		StatusCode: int(codes.OK),
		Headers:    headers,
		Body:       body,
	}, nil
}

func replayGRPCResponse(ctx context.Context, method string, rec *idempotency.Record, cfg IdempotencyConfig) (any, error) {
	_ = grpc.SetTrailer(ctx, gmetadata.Pairs("x-cache-lookup", "HIT", "x-idempotent-replayed", "true"))

	// 1. Check custom ResponseFactory
	if cfg.ResponseFactory != nil {
		target := cfg.ResponseFactory(method)
		if pm, ok := target.(proto.Message); ok {
			if err := proto.Unmarshal(rec.Body, pm); err == nil {
				return pm, nil
			}
		}
		return target, nil
	}

	// 2. Resolve via ProtoReflect GlobalTypes registry
	if protoType, ok := rec.Headers["proto_type"]; ok && protoType != "" {
		mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(protoType))
		if err == nil && mt != nil {
			msg := mt.New().Interface()
			if uErr := proto.Unmarshal(rec.Body, msg); uErr == nil {
				return msg, nil
			}
		}
	}

	return nil, nil
}

func handleGRPCExecutionError(err error) error {
	if errors.Is(err, idempotency.ErrTimeoutWaitingInflight) {
		return status.Error(codes.Aborted, "idempotency lock timeout exceeded")
	}
	if errors.Is(err, idempotency.ErrInflightFailed) {
		return status.Error(codes.Aborted, "concurrent request failed, please retry")
	}
	return err
}
