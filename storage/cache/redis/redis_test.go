package redis_test

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/storage/cache"
	rediscache "github.com/agilsyofian/gosdk/storage/cache/redis"
)

func TestInterfaceImplementation(t *testing.T) {
	var _ cache.Cache = (*rediscache.Cache)(nil)
}

func TestMGet_EmptyKeys(t *testing.T) {
	c, err := rediscache.NewRedisCache(nil, rediscache.Config{
		Addrs: []string{"127.0.0.1:6379"},
	})
	if err != nil {
		t.Fatalf("failed to init redis cache: %v", err)
	}

	// Calling MGet with empty keys should return empty map immediately without connecting
	res, err := c.MGet(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on empty keys, got: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty result map, got: %v", res)
	}
}
