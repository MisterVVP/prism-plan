package subscription

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"stream-service/domain"
	"stream-service/internal/consts"
)

type fakeStore struct {
	tasks  []domain.Task
	called int
}

func (f *fakeStore) FetchTasks(ctx context.Context, userID string) ([]domain.Task, error) {
	f.called++
	return f.tasks, nil
}

func TestSubscribeUpdates(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer rc.Close()
	store := &fakeStore{tasks: []domain.Task{{ID: "1", Title: "t"}}}
	var mu sync.Mutex
	var gotUID string
	var gotData []byte
	broadcast := func(uid string, data []byte) {
		mu.Lock()
		gotUID = uid
		gotData = data
		mu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		SubscribeUpdates(ctx, echo.New().Logger, rc, store, "chan", broadcast)
		close(done)
	}()
	// wait for subscription to start
	time.Sleep(50 * time.Millisecond)
	payload := `{"UserId":"user1"}`
	if err := rc.Publish(context.Background(), "chan", payload).Err(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	uid := gotUID
	data := gotData
	mu.Unlock()
	expectedData, _ := json.Marshal(store.tasks)
	if uid != "user1" {
		t.Fatalf("expected user1, got %s", uid)
	}
	if string(data) != string(expectedData) {
		t.Fatalf("unexpected data %s", string(data))
	}
	if val := rc.Get(context.Background(), consts.TasksKeyPrefix+"user1").Val(); val != string(expectedData) {
		t.Fatalf("expected cache %s, got %s", string(expectedData), val)
	}
	if store.called != 1 {
		t.Fatalf("expected FetchTasks once, got %d", store.called)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SubscribeUpdates did not exit")
	}
}
