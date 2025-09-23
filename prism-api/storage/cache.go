package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
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

	mu               sync.Mutex
	taskSnapshots    map[string]string
	settingSnapshots map[string]string
	pendingTasks     map[string]string
	pendingSettings  map[string]string
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
		base:             base,
		redis:            client,
		ttl:              ttl,
		taskSnapshots:    make(map[string]string),
		settingSnapshots: make(map[string]string),
		pendingTasks:     make(map[string]string),
		pendingSettings:  make(map[string]string),
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

	c.markPending(userID)
	c.evict(ctx, userID)
	return nil
}

func (c *Cache) markPending(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pendingTasks[userID] = c.taskSnapshots[userID]
	c.pendingSettings[userID] = c.settingSnapshots[userID]
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
	data, err := json.Marshal(tasks)
	if err != nil {
		return
	}
	if !c.shouldStoreTasks(userID, data) {
		return
	}
	if c.redis == nil || c.ttl == 0 {
		return
	}
	_ = c.redis.Set(ctx, tasksCacheKey(userID), data, c.ttl).Err()
}

func (c *Cache) storeSettings(ctx context.Context, userID string, settings domain.Settings) {
	data, err := json.Marshal(settings)
	if err != nil {
		return
	}
	if !c.shouldStoreSettings(userID, data) {
		return
	}
	if c.redis == nil || c.ttl == 0 {
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

func (c *Cache) shouldStoreTasks(userID string, data []byte) bool {
	hash := hashBytes(data)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.taskSnapshots[userID] = hash

	if baseline, pending := c.pendingTasks[userID]; pending {
		if baseline == "" {
			c.pendingTasks[userID] = hash
			return false
		}
		if baseline == hash {
			return false
		}
		delete(c.pendingTasks, userID)
	}
	return true
}

func (c *Cache) shouldStoreSettings(userID string, data []byte) bool {
	hash := hashBytes(data)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.settingSnapshots[userID] = hash

	if baseline, pending := c.pendingSettings[userID]; pending {
		if baseline == "" {
			c.pendingSettings[userID] = hash
			return false
		}
		if baseline == hash {
			return false
		}
		delete(c.pendingSettings, userID)
	}
	return true
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
