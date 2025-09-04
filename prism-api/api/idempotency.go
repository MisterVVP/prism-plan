package api

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type processedKey struct {
	userID   string
	key      string
	entityID string
	typ      string
}

type Deduper interface {
	Add(ctx context.Context, pk processedKey) (bool, error)
	Remove(ctx context.Context, pk processedKey) error
}

type RedisDeduper struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisDeduper(client *redis.Client, ttl time.Duration) *RedisDeduper {
	return &RedisDeduper{client: client, ttl: ttl}
}

func (r *RedisDeduper) key(pk processedKey) string {
	return fmt.Sprintf("%s:%s:%s:%s", pk.userID, pk.key, pk.entityID, pk.typ)
}

func (r *RedisDeduper) Add(ctx context.Context, pk processedKey) (bool, error) {
	return r.client.SetNX(ctx, r.key(pk), 1, r.ttl).Result()
}

func (r *RedisDeduper) Remove(ctx context.Context, pk processedKey) error {
	return r.client.Del(ctx, r.key(pk)).Err()
}
