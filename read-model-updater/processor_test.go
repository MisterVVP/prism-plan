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

func TestProcessEventPublishesUpdate(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	orch := &fakeOrchestrator{}
	ctx := context.Background()

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
	if err := processEvent(ctx, orch, rc, "tasks", "settings", ev, payload); err != nil {
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
}
