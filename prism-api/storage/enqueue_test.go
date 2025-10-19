package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
	"prism-api/domain"
)

type fakeQueue struct {
	mu       sync.Mutex
	inFlight int
	max      int
	count    int
	failAt   int
	sleep    time.Duration
}

func newFakeQueue() *fakeQueue {
	return &fakeQueue{failAt: -1, sleep: 1 * time.Millisecond}
}

func (f *fakeQueue) EnqueueMessage(ctx context.Context, content string, o *azqueue.EnqueueMessageOptions) (azqueue.EnqueueMessagesResponse, error) {
	f.mu.Lock()
	idx := f.count
	f.count++
	f.inFlight++
	if f.inFlight > f.max {
		f.max = f.inFlight
	}
	f.mu.Unlock()

	if f.sleep > 0 {
		select {
		case <-time.After(f.sleep):
		case <-ctx.Done():
			f.mu.Lock()
			f.inFlight--
			f.mu.Unlock()
			return azqueue.EnqueueMessagesResponse{}, ctx.Err()
		}
	}

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()

	if f.failAt >= 0 && idx == f.failAt {
		return azqueue.EnqueueMessagesResponse{}, errors.New("enqueue failure")
	}

	return azqueue.EnqueueMessagesResponse{}, nil
}

func (f *fakeQueue) GetProperties(ctx context.Context, o *azqueue.GetQueuePropertiesOptions) (azqueue.GetQueuePropertiesResponse, error) {
	return azqueue.GetQueuePropertiesResponse{}, nil
}

func TestEnqueueCommandsUsesConcurrency(t *testing.T) {
	fq := newFakeQueue()
	store := &Storage{
		commandQueue:     fq,
		queueConcurrency: 4,
	}
	cmds := make([]domain.Command, 8)
	for i := range cmds {
		cmds[i] = domain.Command{IdempotencyKey: "k"}
	}

	if err := store.EnqueueCommands(context.Background(), "user", cmds); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if fq.max < 2 {
		t.Fatalf("expected concurrent sends, max in flight: %d", fq.max)
	}
	if fq.count != len(cmds) {
		t.Fatalf("expected %d sends, got %d", len(cmds), fq.count)
	}
}

func TestEnqueueCommandsPropagatesErrors(t *testing.T) {
	fq := newFakeQueue()
	fq.failAt = 2
	store := &Storage{
		commandQueue:     fq,
		queueConcurrency: 3,
	}
	cmds := make([]domain.Command, 6)
	for i := range cmds {
		cmds[i] = domain.Command{IdempotencyKey: "k"}
	}

	err := store.EnqueueCommands(context.Background(), "user", cmds)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnqueueCommandsSequentialWhenConfigured(t *testing.T) {
	fq := newFakeQueue()
	store := &Storage{
		commandQueue:     fq,
		queueConcurrency: 1,
	}
	cmds := make([]domain.Command, 5)
	for i := range cmds {
		cmds[i] = domain.Command{IdempotencyKey: "k"}
	}

	if err := store.EnqueueCommands(context.Background(), "user", cmds); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if fq.max != 1 {
		t.Fatalf("expected sequential sends, observed max in flight: %d", fq.max)
	}
}
