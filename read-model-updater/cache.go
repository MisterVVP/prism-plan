package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"read-model-updater/domain"
)

type cacheStore interface {
	ListTasksPage(ctx context.Context, userID string, limit int32, nextPartitionKey, nextRowKey *string) ([]domain.TaskEntity, *string, *string, error)
	GetUserSettings(ctx context.Context, id string) (*domain.UserSettingsEntity, error)
}

type cacheRefresher interface {
	RefreshTasks(ctx context.Context, userID string, entityID string, lastUpdated int64)
	RefreshSettings(ctx context.Context, userID string, lastUpdated int64)
}

type cacheUpdater struct {
	store        cacheStore
	redis        *redis.Client
	tasksTTL     time.Duration
	settingsTTL  time.Duration
	tasksPerPage int32
	cachedPages  int
	now          func() time.Time
}

const (
	tasksCachePrefix    = "ts"
	settingsCachePrefix = "us"
)

type cachedTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Notes    string `json:"notes,omitempty"`
	Category string `json:"category"`
	Order    int    `json:"order"`
	Done     bool   `json:"done,omitempty"`
}

type cachedTasks struct {
	Version       int          `json:"version"`
	CachedAt      time.Time    `json:"cachedAt"`
	LastUpdatedAt int64        `json:"lastUpdatedAt"`
	PageSize      int          `json:"pageSize"`
	CachedPages   int          `json:"cachedPages,omitempty"`
	PageTokens    []string     `json:"pageTokens,omitempty"`
	NextPageToken string       `json:"nextPageToken,omitempty"`
	Tasks         []cachedTask `json:"tasks"`
}

type cachedSettings struct {
	Version       int                 `json:"version"`
	CachedAt      time.Time           `json:"cachedAt"`
	LastUpdatedAt int64               `json:"lastUpdatedAt"`
	Settings      cachedSettingsEntry `json:"settings"`
}

type cachedSettingsEntry struct {
	TasksPerCategory int  `json:"tasksPerCategory"`
	ShowDoneTasks    bool `json:"showDoneTasks"`
}

func newCacheUpdater(store cacheStore, redis *redis.Client, tasksPerPage int32, cachedPages int, tasksTTL, settingsTTL time.Duration) *cacheUpdater {
	if tasksPerPage <= 0 {
		tasksPerPage = 1
	}
	if cachedPages <= 0 {
		cachedPages = 1
	}
	if tasksTTL <= 0 {
		tasksTTL = 12 * time.Hour
	}
	if settingsTTL <= 0 {
		settingsTTL = 4 * time.Hour
	}
	return &cacheUpdater{
		store:        store,
		redis:        redis,
		tasksTTL:     tasksTTL,
		settingsTTL:  settingsTTL,
		tasksPerPage: tasksPerPage,
		cachedPages:  cachedPages,
		now:          time.Now,
	}
}

func (c *cacheUpdater) RefreshTasks(ctx context.Context, userID string, entityID string, lastUpdated int64) {
	if c == nil || c.redis == nil || c.store == nil {
		return
	}
	pageTokens := make([]string, 0, c.cachedPages-1)
	entries := make([]cachedTask, 0)
	maxTs := lastUpdated
	foundEntity := entityID == ""
	var nextToken string
	var nextPK *string
	var nextRK *string
	for page := 0; page < c.cachedPages; page++ {
		limit := c.tasksPerPage
		tasks, pk, rk, err := c.store.ListTasksPage(ctx, userID, limit, nextPK, nextRK)
		if err != nil {
			log.WithError(err).WithField("user", userID).Error("failed to list tasks for cache")
			c.purgeTasksCache(ctx, userID)
			return
		}
		if len(tasks) == 0 {
			token, encErr := encodeContinuationToken(pk, rk)
			if encErr != nil {
				log.WithError(encErr).WithField("user", userID).Error("failed to encode continuation token for cache")
				c.purgeTasksCache(ctx, userID)
			} else {
				nextToken = token
			}
			break
		}
		for _, t := range tasks {
			entries = append(entries, cachedTask{
				ID:       t.RowKey,
				Title:    t.Title,
				Notes:    t.Notes,
				Category: t.Category,
				Order:    t.Order,
				Done:     t.Done,
			})
			if t.EventTimestamp > maxTs {
				maxTs = t.EventTimestamp
			}
			if !foundEntity && t.RowKey == entityID {
				foundEntity = true
			}
		}
		token, encErr := encodeContinuationToken(pk, rk)
		if encErr != nil {
			log.WithError(encErr).WithField("user", userID).Error("failed to encode continuation token for cache")
			c.purgeTasksCache(ctx, userID)
			return
		}
		nextToken = token
		if page < c.cachedPages-1 && token != "" && len(tasks) == int(limit) {
			pageTokens = append(pageTokens, token)
		}
		if len(tasks) < int(limit) || pk == nil || rk == nil {
			break
		}
		nextPK = pk
		nextRK = rk
	}
	if !foundEntity {
		log.WithFields(log.Fields{"user": userID, "task": entityID}).Warn("cache refresh missing entity; purging entry")
		key := cacheKey(userID, tasksCachePrefix)
		if err := c.redis.Del(ctx, key).Err(); err != nil {
			log.WithError(err).WithField("user", userID).Error("failed to delete tasks cache entry")
		}
		return
	}
	pageSize := int(c.tasksPerPage)
	cachedPages := 0
	if len(entries) > 0 && pageSize > 0 {
		cachedPages = (len(entries) + pageSize - 1) / pageSize
		if cachedPages > c.cachedPages {
			cachedPages = c.cachedPages
		}
	}
	payload := cachedTasks{
		Version:       1,
		CachedAt:      c.now().UTC(),
		LastUpdatedAt: maxTs,
		PageSize:      pageSize,
		CachedPages:   cachedPages,
		PageTokens:    pageTokens,
		NextPageToken: nextToken,
		Tasks:         entries,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.WithError(err).WithField("user", userID).Error("failed to marshal tasks cache payload")
		c.purgeTasksCache(ctx, userID)
		return
	}
	if err := c.redis.Set(ctx, cacheKey(userID, tasksCachePrefix), data, c.tasksTTL).Err(); err != nil {
		log.WithError(err).WithField("user", userID).Error("failed to store tasks cache entry")
		c.purgeTasksCache(ctx, userID)
	}
}

func (c *cacheUpdater) purgeTasksCache(ctx context.Context, userID string) {
	if c == nil || c.redis == nil {
		return
	}
	if err := c.redis.Del(ctx, cacheKey(userID, tasksCachePrefix)).Err(); err != nil {
		log.WithError(err).WithField("user", userID).Error("failed to delete tasks cache entry")
	}
}

func (c *cacheUpdater) RefreshSettings(ctx context.Context, userID string, lastUpdated int64) {
	if c == nil || c.redis == nil || c.store == nil {
		return
	}
	ent, err := c.store.GetUserSettings(ctx, userID)
	if err != nil {
		log.WithError(err).WithField("user", userID).Error("failed to load settings for cache")
		return
	}
	key := cacheKey(userID, settingsCachePrefix)
	if ent == nil {
		if err := c.redis.Del(ctx, key).Err(); err != nil {
			log.WithError(err).WithField("user", userID).Error("failed to delete settings cache entry")
		}
		return
	}
	ts := ent.EventTimestamp
	if ts < lastUpdated {
		ts = lastUpdated
	}
	payload := cachedSettings{
		Version:       1,
		CachedAt:      c.now().UTC(),
		LastUpdatedAt: ts,
		Settings: cachedSettingsEntry{
			TasksPerCategory: ent.TasksPerCategory,
			ShowDoneTasks:    ent.ShowDoneTasks,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.WithError(err).WithField("user", userID).Error("failed to marshal settings cache payload")
		return
	}
	if err := c.redis.Set(ctx, key, data, c.settingsTTL).Err(); err != nil {
		log.WithError(err).WithField("user", userID).Error("failed to store settings cache entry")
	}
}

func cacheKey(userID, prefix string) string {
	return userID + ":" + prefix
}

func encodeContinuationToken(partitionKey, rowKey *string) (string, error) {
	if partitionKey == nil || rowKey == nil {
		return "", nil
	}
	if len(*partitionKey) == 0 || len(*rowKey) == 0 {
		return "", nil
	}
	pk := []byte(*partitionKey)
	rk := []byte(*rowKey)
	data := make([]byte, 8+len(pk)+len(rk))
	binary.BigEndian.PutUint32(data[0:4], uint32(len(pk)))
	binary.BigEndian.PutUint32(data[4:8], uint32(len(rk)))
	copy(data[8:], pk)
	copy(data[8+len(pk):], rk)
	return base64.RawURLEncoding.EncodeToString(data), nil
}
