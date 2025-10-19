package api

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"prism-api/domain"
)

type outboxTestStore struct {
	block chan struct{}
	ch    chan []domain.Command
}

func newOutboxTestStore() *outboxTestStore {
	return &outboxTestStore{ch: make(chan []domain.Command, 8)}
}

func (s *outboxTestStore) FetchTasks(context.Context, string, string) ([]domain.Task, string, error) {
	return nil, "", nil
}

func (s *outboxTestStore) FetchSettings(context.Context, string) (domain.Settings, error) {
	return domain.Settings{}, nil
}

func (s *outboxTestStore) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	if s.block != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.block:
		}
	}
	cpy := make([]domain.Command, len(cmds))
	copy(cpy, cmds)
	select {
	case s.ch <- cpy:
	default:
	}
	return nil
}

type stubDeduper struct{}

func (stubDeduper) Add(context.Context, string, string) (bool, error) { return true, nil }
func (stubDeduper) Remove(context.Context, string, string) error      { return nil }

func configureOutboxEnv(t *testing.T, dir string, buffer, workers, batch int, handoff time.Duration) {
	t.Helper()
	os.Setenv("OUTBOX_DIR", dir)
	os.Setenv("OUTBOX_BUFFER", itoa(buffer))
	os.Setenv("OUTBOX_WORKERS", itoa(workers))
	os.Setenv("OUTBOX_BATCH", itoa(batch))
	os.Setenv("OUTBOX_HANDOFF_TIMEOUT", handoff.String())
	os.Setenv("OUTBOX_SYNC_EVERY", "1")
	os.Setenv("OUTBOX_SYNC_INTERVAL", "0")
	os.Setenv("OUTBOX_RETRY_INITIAL", "10ms")
	os.Setenv("OUTBOX_RETRY_MAX", "100ms")
}

func clearOutboxEnvVars() {
	keys := []string{
		"OUTBOX_DIR", "OUTBOX_BUFFER", "OUTBOX_WORKERS", "OUTBOX_BATCH",
		"OUTBOX_HANDOFF_TIMEOUT", "OUTBOX_SYNC_EVERY", "OUTBOX_SYNC_INTERVAL",
		"OUTBOX_RETRY_INITIAL", "OUTBOX_RETRY_MAX", "OUTBOX_SEGMENT_MB",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func TestCommandOutboxProcessesCommands(t *testing.T) {
	t.Cleanup(func() {
		shutdownCommandSender()
		clearOutboxEnvVars()
	})
	dir := t.TempDir()
	configureOutboxEnv(t, dir, 8, 2, 2, 25*time.Millisecond)

	store := newOutboxTestStore()
	logger := log.New()
	initCommandSender(store, stubDeduper{}, logger)

	job := enqueueJob{userID: "user", cmds: []domain.Command{{IdempotencyKey: "a", ID: "a"}}}
	if err := enqueueCommands(job); err != nil {
		t.Fatalf("enqueueCommands returned error: %v", err)
	}

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("command was not drained")
	case cmds := <-store.ch:
		if len(cmds) != 1 || cmds[0].IdempotencyKey != "a" {
			t.Fatalf("unexpected commands: %#v", cmds)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		stats, err := getOutboxStats()
		if err == nil && stats.Delivered >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("outbox stats did not report delivery")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCommandOutboxSaturation(t *testing.T) {
	t.Cleanup(func() {
		shutdownCommandSender()
		clearOutboxEnvVars()
	})
	dir := t.TempDir()
	configureOutboxEnv(t, dir, 1, 1, 1, 10*time.Millisecond)

	store := newOutboxTestStore()
	block := make(chan struct{})
	store.block = block
	logger := log.New()
	initCommandSender(store, stubDeduper{}, logger)

	job := enqueueJob{userID: "u", cmds: []domain.Command{{IdempotencyKey: "k1", ID: "k1"}}}
	if err := enqueueCommands(job); err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	job2 := enqueueJob{userID: "u", cmds: []domain.Command{{IdempotencyKey: "k2", ID: "k2"}}}
	if err := enqueueCommands(job2); err != nil {
		t.Fatalf("second enqueue failed: %v", err)
	}
	job3 := enqueueJob{userID: "u", cmds: []domain.Command{{IdempotencyKey: "k3", ID: "k3"}}}
	if err := enqueueCommands(job3); !errors.Is(err, errOutboxSaturated) {
		t.Fatalf("expected saturation error, got %v", err)
	}

	close(block)

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("commands not drained after releasing block")
	case <-store.ch:
	}
}
