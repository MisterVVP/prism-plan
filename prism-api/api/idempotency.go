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

// AddMany attempts to add the provided keys in a single Redis pipeline and
// returns a boolean slice indicating which keys were newly recorded. When an
// error occurs, the slice contains the results for commands processed before
// the failure so callers may roll back any successful additions.
func (r *RedisDeduper) AddMany(ctx context.Context, userID string, keys []string) ([]bool, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	results := make([]bool, len(keys))
	cmds, err := r.client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, key := range keys {
			pipe.SetNX(ctx, r.key(userID, key), 1, r.ttl)
		}
		return nil
	})
	if err != nil {
		return results, err
	}
	if len(cmds) != len(keys) {
		return results, fmt.Errorf("deduper pipeline mismatch: expected %d results, got %d", len(keys), len(cmds))
	}
	for i, cmd := range cmds {
		boolCmd, ok := cmd.(*redis.BoolCmd)
		if !ok {
			return results, fmt.Errorf("unexpected redis response type %T", cmd)
		}
		val, cmdErr := boolCmd.Result()
		if cmdErr != nil {
			return results, cmdErr
		}
		results[i] = val
	}
	return results, nil
}
