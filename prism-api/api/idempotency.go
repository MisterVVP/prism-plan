package api

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisDeduper stores processed idempotency keys in Redis so all instances
// can avoid reprocessing the same command.
type RedisDeduper struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisDeduper creates a deduper using the provided Redis client and TTL.
func NewRedisDeduper(client *redis.Client, ttl time.Duration) *RedisDeduper {
	return &RedisDeduper{client: client, ttl: ttl}
}

func (r *RedisDeduper) key(userID, key string) string {
	return fmt.Sprintf("%s:%s", userID, key)
}

// Add records the key if it does not already exist. It returns true when the
// key was newly added.
func (r *RedisDeduper) Add(ctx context.Context, userID, key string) (bool, error) {
	return r.client.SetNX(ctx, r.key(userID, key), 1, r.ttl).Result()
}

// Remove deletes a previously recorded key. It is used when downstream
// processing fails so the caller may retry the command.
func (r *RedisDeduper) Remove(ctx context.Context, userID, key string) error {
	return r.client.Del(ctx, r.key(userID, key)).Err()
}
