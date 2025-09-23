package storage

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"prism-api/domain"
)

type stubBackend struct {
	fetchTasksFn      func(ctx context.Context, userID string) ([]domain.Task, error)
	fetchSettingsFn   func(ctx context.Context, userID string) (domain.Settings, error)
	enqueueCommandsFn func(ctx context.Context, userID string, cmds []domain.Command) error
}

func (s *stubBackend) FetchTasks(ctx context.Context, userID string) ([]domain.Task, error) {
	if s.fetchTasksFn == nil {
		return nil, errors.New("unexpected FetchTasks call")
	}
	return s.fetchTasksFn(ctx, userID)
}

func (s *stubBackend) FetchSettings(ctx context.Context, userID string) (domain.Settings, error) {
	if s.fetchSettingsFn == nil {
		return domain.Settings{}, errors.New("unexpected FetchSettings call")
	}
	return s.fetchSettingsFn(ctx, userID)
}

func (s *stubBackend) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	if s.enqueueCommandsFn == nil {
		return errors.New("unexpected EnqueueCommands call")
	}
	return s.enqueueCommandsFn(ctx, userID, cmds)
}

func TestCacheFetchTasksMissThenHit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	userID := "user-1"
	expected := []domain.Task{{ID: "t1", Title: "Write code"}}

	var calls int
	cache := NewCache(&stubBackend{
		fetchTasksFn: func(ctx context.Context, uid string) ([]domain.Task, error) {
			calls++
			if uid != userID {
				t.Fatalf("unexpected user id: %s", uid)
			}
			return append([]domain.Task(nil), expected...), nil
		},
	}, client, time.Minute)

	tasks, err := cache.FetchTasks(ctx, userID)
	if err != nil {
		t.Fatalf("fetch tasks: %v", err)
	}
	if !reflect.DeepEqual(tasks, expected) {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call to backend, got %d", calls)
	}
	if ttl := mr.TTL(tasksCacheKey(userID)); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("unexpected TTL: %v", ttl)
	}

	cached, err := cache.FetchTasks(ctx, userID)
	if err != nil {
		t.Fatalf("fetch cached tasks: %v", err)
	}
	if !reflect.DeepEqual(cached, expected) {
		t.Fatalf("unexpected cached tasks: %#v", cached)
	}
	if calls != 1 {
		t.Fatalf("expected cached fetch to avoid backend, calls=%d", calls)
	}
}

func TestCacheFetchSettingsMissThenHit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	userID := "user-settings"
	expected := domain.Settings{TasksPerCategory: 3, ShowDoneTasks: true}

	var calls int
	cache := NewCache(&stubBackend{
		fetchSettingsFn: func(ctx context.Context, uid string) (domain.Settings, error) {
			calls++
			if uid != userID {
				t.Fatalf("unexpected user id: %s", uid)
			}
			return expected, nil
		},
	}, client, time.Minute)

	settings, err := cache.FetchSettings(ctx, userID)
	if err != nil {
		t.Fatalf("fetch settings: %v", err)
	}
	if settings != expected {
		t.Fatalf("unexpected settings: %#v", settings)
	}
	if calls != 1 {
		t.Fatalf("expected 1 backend call, got %d", calls)
	}
	if ttl := mr.TTL(settingsCacheKey(userID)); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("unexpected TTL: %v", ttl)
	}

	cached, err := cache.FetchSettings(ctx, userID)
	if err != nil {
		t.Fatalf("fetch cached settings: %v", err)
	}
	if cached != expected {
		t.Fatalf("unexpected cached settings: %#v", cached)
	}
	if calls != 1 {
		t.Fatalf("expected cached fetch to avoid backend, calls=%d", calls)
	}
}

func TestCacheEnqueueCommandsEvictsKeys(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	userID := "evict-user"
	if err := client.Set(ctx, tasksCacheKey(userID), []byte("[]"), time.Hour).Err(); err != nil {
		t.Fatalf("seed tasks cache: %v", err)
	}
	if err := client.Set(ctx, settingsCacheKey(userID), []byte(`{"tasksPerCategory":5}`), time.Hour).Err(); err != nil {
		t.Fatalf("seed settings cache: %v", err)
	}

	var calls int
	cache := NewCache(&stubBackend{
		enqueueCommandsFn: func(ctx context.Context, uid string, cmds []domain.Command) error {
			calls++
			if uid != userID {
				t.Fatalf("unexpected user id: %s", uid)
			}
			if len(cmds) == 0 {
				t.Fatalf("expected commands")
			}
			return nil
		},
	}, client, time.Minute)

	if err := cache.EnqueueCommands(ctx, userID, []domain.Command{{ID: "cmd"}}); err != nil {
		t.Fatalf("enqueue commands: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected backend enqueue, got %d calls", calls)
	}
	if mr.Exists(tasksCacheKey(userID)) {
		t.Fatalf("tasks cache key should be evicted")
	}
	if mr.Exists(settingsCacheKey(userID)) {
		t.Fatalf("settings cache key should be evicted")
	}
}

func TestCacheEnqueueCommandsErrorPreservesCache(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	userID := "evict-error"
	if err := client.Set(ctx, tasksCacheKey(userID), []byte("[]"), time.Hour).Err(); err != nil {
		t.Fatalf("seed tasks cache: %v", err)
	}
	if err := client.Set(ctx, settingsCacheKey(userID), []byte("{}"), time.Hour).Err(); err != nil {
		t.Fatalf("seed settings cache: %v", err)
	}

	cache := NewCache(&stubBackend{
		enqueueCommandsFn: func(context.Context, string, []domain.Command) error {
			return errors.New("boom")
		},
	}, client, time.Minute)

	if err := cache.EnqueueCommands(ctx, userID, nil); err == nil {
		t.Fatalf("expected enqueue error")
	}
	if !mr.Exists(tasksCacheKey(userID)) {
		t.Fatalf("tasks cache should remain on error")
	}
	if !mr.Exists(settingsCacheKey(userID)) {
		t.Fatalf("settings cache should remain on error")
	}
}
