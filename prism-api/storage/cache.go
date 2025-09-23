package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"prism-api/domain"
)

type backend interface {
	FetchTasks(ctx context.Context, userID string) ([]domain.Task, error)
	FetchSettings(ctx context.Context, userID string) (domain.Settings, error)
	EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error
}

// Cache wraps a Storage instance with Redis-backed caching for read operations.
type Cache struct {
	*Storage
	base  backend
	redis *redis.Client
	ttl   time.Duration
}

// NewCache creates a caching Storage wrapper using the provided Redis client and TTL.
func NewCache(base backend, client *redis.Client, ttl time.Duration) *Cache {
	if base == nil {
		panic("storage.NewCache: base storage is nil")
	}
	if ttl < 0 {
		ttl = 0
	}

	c := &Cache{
		base:  base,
		redis: client,
		ttl:   ttl,
	}
	if s, ok := base.(*Storage); ok {
		c.Storage = s
	}
	return c
}

func (c *Cache) FetchTasks(ctx context.Context, userID string) ([]domain.Task, error) {
	if tasks, ok := c.loadTasksFromCache(ctx, userID); ok {
		return tasks, nil
	}

	tasks, err := c.base.FetchTasks(ctx, userID)
	if err != nil {
		return nil, err
	}

	c.storeTasks(ctx, userID, tasks)
	return tasks, nil
}

func (c *Cache) FetchSettings(ctx context.Context, userID string) (domain.Settings, error) {
	if settings, ok := c.loadSettingsFromCache(ctx, userID); ok {
		return settings, nil
	}

	settings, err := c.base.FetchSettings(ctx, userID)
	if err != nil {
		return domain.Settings{}, err
	}

	c.storeSettings(ctx, userID, settings)
	return settings, nil
}

func (c *Cache) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	if err := c.base.EnqueueCommands(ctx, userID, cmds); err != nil {
		return err
	}

	c.evict(ctx, userID)
	return nil
}

func (c *Cache) loadTasksFromCache(ctx context.Context, userID string) ([]domain.Task, bool) {
	if c.redis == nil {
		return nil, false
	}
	data, err := c.redis.Get(ctx, tasksCacheKey(userID)).Bytes()
	if err != nil {
		if err != redis.Nil {
			// On redis errors fall back to the backing storage without failing.
			_ = c.redis.Del(ctx, tasksCacheKey(userID)).Err()
		}
		return nil, false
	}
	var tasks []domain.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		_ = c.redis.Del(ctx, tasksCacheKey(userID)).Err()
		return nil, false
	}
	return tasks, true
}

func (c *Cache) loadSettingsFromCache(ctx context.Context, userID string) (domain.Settings, bool) {
	if c.redis == nil {
		return domain.Settings{}, false
	}
	data, err := c.redis.Get(ctx, settingsCacheKey(userID)).Bytes()
	if err != nil {
		if err != redis.Nil {
			_ = c.redis.Del(ctx, settingsCacheKey(userID)).Err()
		}
		return domain.Settings{}, false
	}
	var settings domain.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		_ = c.redis.Del(ctx, settingsCacheKey(userID)).Err()
		return domain.Settings{}, false
	}
	return settings, true
}

func (c *Cache) storeTasks(ctx context.Context, userID string, tasks []domain.Task) {
	if c.redis == nil || c.ttl == 0 {
		return
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		return
	}
	_ = c.redis.Set(ctx, tasksCacheKey(userID), data, c.ttl).Err()
}

func (c *Cache) storeSettings(ctx context.Context, userID string, settings domain.Settings) {
	if c.redis == nil || c.ttl == 0 {
		return
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return
	}
	_ = c.redis.Set(ctx, settingsCacheKey(userID), data, c.ttl).Err()
}

func (c *Cache) evict(ctx context.Context, userID string) {
	if c.redis == nil {
		return
	}
	_, _ = c.redis.Del(ctx, tasksCacheKey(userID), settingsCacheKey(userID)).Result()
}

func tasksCacheKey(userID string) string {
	return "tasks:" + userID
}

func settingsCacheKey(userID string) string {
	return "settings:" + userID
}
