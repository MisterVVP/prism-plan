package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"read-model-updater/domain"
)

type stubCacheStore struct {
	tasksResp           []domain.TaskEntity
	nextPK              *string
	nextRK              *string
	tasksErr            error
	settingsResp        *domain.UserSettingsEntity
	settingsErr         error
	lastListTasksLimit  int32
	lastListTasksUserID string
	lastSettingsUserID  string
}

func (s *stubCacheStore) ListTasksPage(ctx context.Context, userID string, limit int32) ([]domain.TaskEntity, *string, *string, error) {
	s.lastListTasksLimit = limit
	s.lastListTasksUserID = userID
	return s.tasksResp, s.nextPK, s.nextRK, s.tasksErr
}

func (s *stubCacheStore) GetUserSettings(ctx context.Context, id string) (*domain.UserSettingsEntity, error) {
	s.lastSettingsUserID = id
	return s.settingsResp, s.settingsErr
}

func TestCacheUpdaterRefreshTasksStoresPayload(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	ctx := context.Background()

	pk := "p"
	rk := "r"
	store := &stubCacheStore{
		tasksResp: []domain.TaskEntity{{
			Entity:         domain.Entity{PartitionKey: "user", RowKey: "task1"},
			Title:          "Task 1",
			Notes:          "Notes",
			Category:       "cat",
			Order:          1,
			Done:           true,
			EventTimestamp: 90,
		}},
		nextPK: &pk,
		nextRK: &rk,
	}
	updater := newCacheUpdater(store, rc, 5, time.Hour, 2*time.Hour)
	freeze := time.Unix(123, 0).UTC()
	updater.now = func() time.Time { return freeze }

	updater.RefreshTasks(ctx, "user", "task1", 42)

	raw, err := rc.Get(ctx, cacheKey("user", tasksCachePrefix)).Result()
	if err != nil {
		t.Fatalf("redis get: %v", err)
	}
	var payload cachedTasks
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.PageSize != 5 {
		t.Fatalf("unexpected page size: %d", payload.PageSize)
	}
	if payload.LastUpdatedAt != 90 {
		t.Fatalf("unexpected lastUpdatedAt: %d", payload.LastUpdatedAt)
	}
	if payload.CachedAt != freeze {
		t.Fatalf("unexpected cachedAt: %v", payload.CachedAt)
	}
	if payload.NextPageToken == "" {
		t.Fatalf("expected next page token")
	}
	if len(payload.Tasks) != 1 || payload.Tasks[0].ID != "task1" || !payload.Tasks[0].Done {
		t.Fatalf("unexpected tasks payload: %+v", payload.Tasks)
	}
	if got := m.TTL(cacheKey("user", tasksCachePrefix)); got <= 0 {
		t.Fatalf("expected ttl to be set, got %v", got)
	}
	if store.lastListTasksLimit != 5 || store.lastListTasksUserID != "user" {
		t.Fatalf("unexpected store usage: limit=%d user=%s", store.lastListTasksLimit, store.lastListTasksUserID)
	}
}

func TestCacheUpdaterRefreshSettingsStoresPayload(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	ctx := context.Background()

	store := &stubCacheStore{
		settingsResp: &domain.UserSettingsEntity{
			Entity:           domain.Entity{PartitionKey: "user", RowKey: "user"},
			TasksPerCategory: 3,
			ShowDoneTasks:    true,
			EventTimestamp:   50,
		},
	}
	updater := newCacheUpdater(store, rc, 4, time.Hour, 30*time.Minute)
	freeze := time.Unix(200, 0).UTC()
	updater.now = func() time.Time { return freeze }

	updater.RefreshSettings(ctx, "user", 70)

	raw, err := rc.Get(ctx, cacheKey("user", settingsCachePrefix)).Result()
	if err != nil {
		t.Fatalf("redis get: %v", err)
	}
	var payload cachedSettings
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.LastUpdatedAt != 70 {
		t.Fatalf("unexpected lastUpdatedAt: %d", payload.LastUpdatedAt)
	}
	if payload.Settings.TasksPerCategory != 3 || !payload.Settings.ShowDoneTasks {
		t.Fatalf("unexpected settings payload: %+v", payload.Settings)
	}
	if got := m.TTL(cacheKey("user", settingsCachePrefix)); got <= 0 {
		t.Fatalf("expected ttl to be set for settings, got %v", got)
	}
	if store.lastSettingsUserID != "user" {
		t.Fatalf("unexpected settings user: %s", store.lastSettingsUserID)
	}
}

func TestCacheUpdaterRefreshSettingsDeletesMissingEntry(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	ctx := context.Background()

	if err := rc.Set(ctx, cacheKey("user", settingsCachePrefix), "seed", time.Hour).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	store := &stubCacheStore{}
	updater := newCacheUpdater(store, rc, 4, time.Hour, time.Hour)

	updater.RefreshSettings(ctx, "user", 0)

	if _, err := rc.Get(ctx, cacheKey("user", settingsCachePrefix)).Result(); err != redis.Nil {
		t.Fatalf("expected redis nil, got %v", err)
	}
}

func TestCacheUpdaterRefreshTasksDeletesStaleEntryWhenEntityMissing(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	ctx := context.Background()

	if err := rc.Set(ctx, cacheKey("user", tasksCachePrefix), "seed", time.Hour).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}

	store := &stubCacheStore{
		tasksResp: []domain.TaskEntity{{
			Entity:         domain.Entity{PartitionKey: "user", RowKey: "other"},
			Title:          "Task",
			Category:       "cat",
			Order:          1,
			EventTimestamp: 10,
		}},
	}
	updater := newCacheUpdater(store, rc, 5, time.Hour, time.Hour)

	updater.RefreshTasks(ctx, "user", "missing", 99)

	if _, err := rc.Get(ctx, cacheKey("user", tasksCachePrefix)).Result(); err != redis.Nil {
		t.Fatalf("expected cache eviction when entity missing, got %v", err)
	}
}
