package main

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"read-model-updater/domain"
)

type fakeOrchestrator struct{ called bool }

func (f *fakeOrchestrator) Apply(ctx context.Context, ev domain.Event) error {
	f.called = true
	return nil
}

type fakeCache struct {
	tasksRefreshed    bool
	settingsRefreshed bool
}

func (f *fakeCache) RefreshTasks(ctx context.Context, userID string, entityID string, lastUpdated int64) {
	f.tasksRefreshed = true
}

func (f *fakeCache) RefreshSettings(ctx context.Context, userID string, lastUpdated int64) {
	f.settingsRefreshed = true
}

func TestProcessEventPublishesUpdate(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	orch := &fakeOrchestrator{}
	ctx := context.Background()
	cache := &fakeCache{}

	pubsub := rc.Subscribe(ctx, "tasks")
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	done := make(chan string, 1)
	go func() {
		msg := <-pubsub.Channel()
		done <- msg.Payload
	}()

	ev := domain.Event{EntityType: "task", Type: domain.TaskCreated}
	payload := `{"entityType":"task"}`
	if err := processEvent(ctx, orch, cache, rc, "tasks", "settings", ev, payload); err != nil {
		t.Fatalf("processEvent: %v", err)
	}
	select {
	case pl := <-done:
		if pl != payload {
			t.Fatalf("unexpected payload %s", pl)
		}
	case <-time.After(time.Second):
		t.Fatalf("no message received")
	}
	if !orch.called {
		t.Fatalf("orchestrator not called")
	}
	if !cache.tasksRefreshed {
		t.Fatalf("expected task cache refresh")
	}
	if cache.settingsRefreshed {
		t.Fatalf("unexpected settings cache refresh")
	}
}

func TestProcessEventRefreshesSettingsCache(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	orch := &fakeOrchestrator{}
	cache := &fakeCache{}
	ctx := context.Background()

	pubsub := rc.Subscribe(ctx, "settings")
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ev := domain.Event{EntityType: "user-settings", Type: domain.UserSettingsUpdated}
	payload := `{"entityType":"user-settings"}`
	if err := processEvent(ctx, orch, cache, rc, "tasks", "settings", ev, payload); err != nil {
		t.Fatalf("processEvent: %v", err)
	}

	if !cache.settingsRefreshed {
		t.Fatalf("expected settings cache refresh")
	}
	if cache.tasksRefreshed {
		t.Fatalf("unexpected tasks cache refresh")
	}
}
