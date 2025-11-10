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

type taskPageResponse struct {
	tasks  []domain.TaskEntity
	nextPK *string
	nextRK *string
	err    error
}

type listCall struct {
	limit  int32
	userID string
	nextPK string
	nextRK string
}

type stubCacheStore struct {
	taskPages        []taskPageResponse
	settingsResp     *domain.UserSettingsEntity
	settingsErr      error
	listCalls        []listCall
	lastSettingsUser string
}

func (s *stubCacheStore) ListTasksPage(ctx context.Context, userID string, limit int32, nextPartitionKey, nextRowKey *string) ([]domain.TaskEntity, *string, *string, error) {
	call := listCall{limit: limit, userID: userID}
	if nextPartitionKey != nil {
		call.nextPK = *nextPartitionKey
	}
	if nextRowKey != nil {
		call.nextRK = *nextRowKey
	}
	s.listCalls = append(s.listCalls, call)
	if len(s.taskPages) == 0 {
		return []domain.TaskEntity{}, nil, nil, nil
	}
	resp := s.taskPages[0]
	s.taskPages = s.taskPages[1:]
	if resp.err != nil {
		return nil, nil, nil, resp.err
	}
	return resp.tasks, resp.nextPK, resp.nextRK, nil
}

func (s *stubCacheStore) GetUserSettings(ctx context.Context, id string) (*domain.UserSettingsEntity, error) {
	s.lastSettingsUser = id
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

	pk1 := "p1"
	rk1 := "r1"
	pk2 := "p2"
	rk2 := "r2"
	store := &stubCacheStore{
		taskPages: []taskPageResponse{
			{
				tasks: []domain.TaskEntity{{
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "task1"},
					Title:          "Task 1",
					Notes:          "Notes",
					Category:       "cat",
					Order:          1,
					Done:           true,
					EventTimestamp: 90,
				}, {
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "task2"},
					Title:          "Task 2",
					Category:       "cat",
					Order:          2,
					EventTimestamp: 100,
				}, {
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "task3"},
					Title:          "Task 3",
					Category:       "cat",
					Order:          3,
					EventTimestamp: 80,
				}, {
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "task4"},
					Title:          "Task 4",
					Category:       "cat",
					Order:          4,
					EventTimestamp: 70,
				}, {
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "task5"},
					Title:          "Task 5",
					Category:       "cat",
					Order:          5,
					EventTimestamp: 60,
				}},
				nextPK: &pk1,
				nextRK: &rk1,
			},
			{
				tasks: []domain.TaskEntity{{
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "task6"},
					Title:          "Task 6",
					Category:       "cat",
					Order:          6,
					EventTimestamp: 120,
				}},
				nextPK: &pk2,
				nextRK: &rk2,
			},
		},
	}
	updater := newCacheUpdater(store, rc, 5, 2, time.Hour, 2*time.Hour)
	freeze := time.Unix(123, 0).UTC()
	updater.now = func() time.Time { return freeze }

	updater.RefreshTasks(ctx, "user", "task6", 42)

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
	if payload.CachedPages != 2 {
		t.Fatalf("unexpected cachedPages: %d", payload.CachedPages)
	}
	if len(payload.PageTokens) != 1 {
		t.Fatalf("unexpected page tokens: %v", payload.PageTokens)
	}
	tokenPage1, err := encodeContinuationToken(&pk1, &rk1)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	if payload.PageTokens[0] != tokenPage1 {
		t.Fatalf("unexpected first page token: %s", payload.PageTokens[0])
	}
	finalToken, err := encodeContinuationToken(&pk2, &rk2)
	if err != nil {
		t.Fatalf("encode final token: %v", err)
	}
	if payload.NextPageToken != finalToken {
		t.Fatalf("unexpected next page token: %s", payload.NextPageToken)
	}
	if payload.LastUpdatedAt != 120 {
		t.Fatalf("unexpected lastUpdatedAt: %d", payload.LastUpdatedAt)
	}
	if payload.CachedAt != freeze {
		t.Fatalf("unexpected cachedAt: %v", payload.CachedAt)
	}
	if len(payload.Tasks) != 6 || payload.Tasks[0].ID != "task1" || payload.Tasks[5].ID != "task6" {
		t.Fatalf("unexpected tasks payload: %+v", payload.Tasks)
	}
	if got := m.TTL(cacheKey("user", tasksCachePrefix)); got <= 0 {
		t.Fatalf("expected ttl to be set, got %v", got)
	}
	if len(store.listCalls) != 2 {
		t.Fatalf("expected two list calls, got %d", len(store.listCalls))
	}
	if store.listCalls[0].limit != 5 || store.listCalls[0].userID != "user" || store.listCalls[0].nextPK != "" || store.listCalls[0].nextRK != "" {
		t.Fatalf("unexpected first list call: %+v", store.listCalls[0])
	}
	if store.listCalls[1].nextPK != pk1 || store.listCalls[1].nextRK != rk1 {
		t.Fatalf("unexpected continuation: %+v", store.listCalls[1])
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
	updater := newCacheUpdater(store, rc, 4, 1, time.Hour, 30*time.Minute)
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
	if store.lastSettingsUser != "user" {
		t.Fatalf("unexpected settings user: %s", store.lastSettingsUser)
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
	updater := newCacheUpdater(store, rc, 4, 1, time.Hour, time.Hour)

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
		taskPages: []taskPageResponse{
			{
				tasks: []domain.TaskEntity{{
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "other"},
					Title:          "Task",
					Category:       "cat",
					Order:          1,
					EventTimestamp: 10,
				}},
			},
		},
	}
	updater := newCacheUpdater(store, rc, 5, 1, time.Hour, time.Hour)

	updater.RefreshTasks(ctx, "user", "missing", 99)

	if _, err := rc.Get(ctx, cacheKey("user", tasksCachePrefix)).Result(); err != redis.Nil {
		t.Fatalf("expected cache eviction when entity missing, got %v", err)
	}
}
