package redisclient

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

// New creates a Redis client using REDIS_ADDR env var (default localhost:6379)
// and verifies connectivity with a Ping. Returns error on failure so callers
// can log.Fatal, which triggers K8s restartPolicy: OnFailure.
func New(ctx context.Context) (*redis.Client, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed (%s): %w", addr, err)
	}
	return rdb, nil
}
