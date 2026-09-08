package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/redis/go-redis/v9"
)

func TestExtractRedisKey(t *testing.T) {
	tests := []struct {
		name     string
		cmd      redis.Cmder
		expected string
	}{
		{
			name:     "SET command",
			cmd:      redis.NewStatusCmd(context.Background(), "set", "IDEM:auth:password:12345", "val", "ex", 60),
			expected: "IDEM:auth:password:12345",
		},
		{
			name:     "GET command",
			cmd:      redis.NewStringCmd(context.Background(), "get", "IDEM:auth:password:12345"),
			expected: "IDEM:auth:password:12345",
		},
		{
			name:     "HGET command",
			cmd:      redis.NewStringCmd(context.Background(), "hget", "user:profile:100", "email"),
			expected: "user:profile:100",
		},
		{
			name:     "DEL single key",
			cmd:      redis.NewIntCmd(context.Background(), "del", "session:abc"),
			expected: "session:abc",
		},
		{
			name:     "DEL multiple keys",
			cmd:      redis.NewIntCmd(context.Background(), "del", "session:1", "session:2"),
			expected: "session:1, session:2",
		},
		{
			name:     "MGET multiple keys",
			cmd:      redis.NewSliceCmd(context.Background(), "mget", "key1", "key2", "key3"),
			expected: "key1, key2, key3",
		},
		{
			name:     "MSET multiple keys and values",
			cmd:      redis.NewStatusCmd(context.Background(), "mset", "key1", "val1", "key2", "val2"),
			expected: "key1, key2",
		},
		{
			name:     "PING without args",
			cmd:      redis.NewStatusCmd(context.Background(), "ping"),
			expected: "ping",
		},
		{
			name:     "CLIENT LIST admin command",
			cmd:      redis.NewStringCmd(context.Background(), "client", "list"),
			expected: "client",
		},
		{
			name:     "HELLO admin command",
			cmd:      redis.NewCmd(context.Background(), "hello", 3),
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := extractRedisKey(tt.cmd)
			if actual != tt.expected {
				t.Errorf("extractRedisKey() = %q, expected %q", actual, tt.expected)
			}
		})
	}
}

type mockLogger struct {
	lastPayload logger.LogPayload
}

func (m *mockLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {}
func (m *mockLogger) Info(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastPayload = payload
}
func (m *mockLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (m *mockLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastPayload = payload
}

func TestHook_LogKeyAsPath(t *testing.T) {
	mockL := &mockLogger{}
	cfg := Config{}
	cfg.Logger.Enabled = true

	hook := NewHook(mockL, cfg, "test-app")

	cmd := redis.NewStatusCmd(context.Background(), "set", "IDEM:auth:password:xyz", "myvalue")
	hook.log(context.Background(), cmd, time.Now(), nil)

	if mockL.lastPayload.Path != "IDEM:auth:password:xyz" {
		t.Errorf("expected Path to be 'IDEM:auth:password:xyz', got %q", mockL.lastPayload.Path)
	}
	if mockL.lastPayload.Method != "SET" {
		t.Errorf("expected Method to be 'SET', got %q", mockL.lastPayload.Method)
	}
	if mockL.lastPayload.Status != 200 {
		t.Errorf("expected Status to be 200, got %d", mockL.lastPayload.Status)
	}
}

func TestHook_LogStatus_CacheMiss(t *testing.T) {
	mockL := &mockLogger{}
	cfg := Config{}
	cfg.Logger.Enabled = true

	hook := NewHook(mockL, cfg, "test-app")

	cmd := redis.NewStringCmd(context.Background(), "get", "user:session:notfound")
	hook.log(context.Background(), cmd, time.Now(), redis.Nil)

	if mockL.lastPayload.Status != 404 {
		t.Errorf("expected Status 404 for cache miss, got %d", mockL.lastPayload.Status)
	}
	if mockL.lastPayload.Path != "user:session:notfound" {
		t.Errorf("expected Path 'user:session:notfound', got %q", mockL.lastPayload.Path)
	}
	if mockL.lastPayload.Response != "null" {
		t.Errorf("expected Response 'null', got %v", mockL.lastPayload.Response)
	}
}

func TestHook_LogStatus_RedisError(t *testing.T) {
	mockL := &mockLogger{}
	cfg := Config{}
	cfg.Logger.Enabled = true

	hook := NewHook(mockL, cfg, "test-app")

	cmd := redis.NewStringCmd(context.Background(), "get", "user:session:broken")
	hook.log(context.Background(), cmd, time.Now(), errors.New("dial tcp: connection refused"))

	if mockL.lastPayload.Status != 500 {
		t.Errorf("expected Status 500 for redis error, got %d", mockL.lastPayload.Status)
	}
	if mockL.lastPayload.Error != "dial tcp: connection refused" {
		t.Errorf("expected Error 'dial tcp: connection refused', got %v", mockL.lastPayload.Error)
	}
}

