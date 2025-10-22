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

type stubTaskPage struct {
	tasks  []domain.TaskEntity
	nextPK *string
	nextRK *string
	err    error
}

type stubCacheStore struct {
	taskPages          []stubTaskPage
	settingsResp       *domain.UserSettingsEntity
	settingsErr        error
	lastListTasksLimit []int32
	lastListTasksUser  []string
	lastSettingsUserID string
	calls              int
	defaultErr         error
}

func (s *stubCacheStore) ListTasksPage(ctx context.Context, userID string, limit int32, _ *string, _ *string) ([]domain.TaskEntity, *string, *string, error) {
	s.lastListTasksLimit = append(s.lastListTasksLimit, limit)
	s.lastListTasksUser = append(s.lastListTasksUser, userID)
	defer func() { s.calls++ }()
	if s.calls < len(s.taskPages) {
		page := s.taskPages[s.calls]
		if page.err != nil {
			return nil, nil, nil, page.err
		}
		return page.tasks, page.nextPK, page.nextRK, nil
	}
	if s.defaultErr != nil {
		return nil, nil, nil, s.defaultErr
	}
	return []domain.TaskEntity{}, nil, nil, nil
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
		taskPages: []stubTaskPage{
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
					Done:           false,
					EventTimestamp: 120,
				}},
				nextPK: &pk,
				nextRK: &rk,
			},
			{
				tasks: []domain.TaskEntity{{
					Entity:         domain.Entity{PartitionKey: "user", RowKey: "task3"},
					Title:          "Task 3",
					Category:       "cat",
					Order:          3,
					Done:           true,
					EventTimestamp: 150,
				}},
				nextPK: nil,
				nextRK: nil,
			},
		},
	}
	updater := newCacheUpdater(store, rc, 2, 5, time.Hour, 2*time.Hour)
	freeze := time.Unix(123, 0).UTC()
	updater.now = func() time.Time { return freeze }

	updater.RefreshTasks(ctx, "user", 42)

	raw, err := rc.Get(ctx, cacheKey("user", tasksCachePrefix)).Result()
	if err != nil {
		t.Fatalf("redis get: %v", err)
	}
	var payload cachedTasks
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.PageSize != 2 {
		t.Fatalf("unexpected page size: %d", payload.PageSize)
	}
	if payload.LastUpdatedAt != 150 {
		t.Fatalf("unexpected lastUpdatedAt: %d", payload.LastUpdatedAt)
	}
	if payload.CachedAt != freeze {
		t.Fatalf("unexpected cachedAt: %v", payload.CachedAt)
	}
	firstPage, ok := payload.Pages[""]
	if !ok {
		t.Fatalf("missing first page entry")
	}
	expectedToken, err := encodeContinuationToken(&pk, &rk)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	if firstPage.NextPageToken != expectedToken {
		t.Fatalf("unexpected first page token: %s", firstPage.NextPageToken)
	}
	if len(firstPage.Tasks) != 2 || firstPage.Tasks[0].ID != "task1" || !firstPage.Tasks[0].Done {
		t.Fatalf("unexpected first page payload: %+v", firstPage.Tasks)
	}
	if secondPage, ok := payload.Pages[expectedToken]; !ok {
		t.Fatalf("missing second page entry")
	} else {
		if secondPage.NextPageToken != "" {
			t.Fatalf("expected empty next token for final page, got %s", secondPage.NextPageToken)
		}
		if len(secondPage.Tasks) != 1 || secondPage.Tasks[0].ID != "task3" {
			t.Fatalf("unexpected second page payload: %+v", secondPage.Tasks)
		}
	}
	if got := m.TTL(cacheKey("user", tasksCachePrefix)); got <= 0 {
		t.Fatalf("expected ttl to be set, got %v", got)
	}
	if len(store.lastListTasksLimit) != 2 || store.lastListTasksLimit[0] != 2 || store.lastListTasksLimit[1] != 2 {
		t.Fatalf("unexpected store limits: %#v", store.lastListTasksLimit)
	}
	for _, user := range store.lastListTasksUser {
		if user != "user" {
			t.Fatalf("unexpected store user: %s", user)
		}
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
			Entity:             domain.Entity{PartitionKey: "user", RowKey: "user"},
			TasksPerCategory:   3,
			ShowDoneTasks:      true,
			EventTimestamp:     50,
			EventTimestampType: "Edm.Int64",
		},
	}
	updater := newCacheUpdater(store, rc, 4, 4, time.Hour, 30*time.Minute)
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
	updater := newCacheUpdater(store, rc, 4, 4, time.Hour, time.Hour)

	updater.RefreshSettings(ctx, "user", 0)

	if _, err := rc.Get(ctx, cacheKey("user", settingsCachePrefix)).Result(); err != redis.Nil {
		t.Fatalf("expected redis nil, got %v", err)
	}
}
