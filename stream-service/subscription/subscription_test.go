package subscription

import (
	"context"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"stream-service/internal/consts"
)

func TestSubscribeUpdates(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer m.Close()
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer rc.Close()

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
		SubscribeUpdates(ctx, echo.New().Logger, rc, "chan", broadcast)
		close(done)
	}()
	// wait for subscription to start
	time.Sleep(50 * time.Millisecond)
	payload := `{"UserId":"user1","msg":"hi"}`
	if err := rc.Publish(context.Background(), "chan", payload).Err(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	uid := gotUID
	data := string(gotData)
	mu.Unlock()
	if uid != "user1" {
		t.Fatalf("expected user1, got %s", uid)
	}
	if data != payload {
		t.Fatalf("unexpected data %s", data)
	}
	if val := rc.Get(context.Background(), consts.TasksKeyPrefix+"user1").Val(); val != payload {
		t.Fatalf("expected cache %s, got %s", payload, val)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SubscribeUpdates did not exit")
	}
}
