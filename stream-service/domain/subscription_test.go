package domain

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
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
	payload := `{"Id":"1","EntityId":"t1","EntityType":"task","Type":"task-created","Data":{"title":"task1","category":"cat","order":1},"Timestamp":123,"UserId":"user1"}`
	if err := rc.Publish(context.Background(), "chan", payload).Err(); err != nil {
		t.Fatalf("publish: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	uid := gotUID
	data := gotData
	mu.Unlock()
	if uid != "user1" {
		t.Fatalf("expected user1, got %s", uid)
	}
	var payloadObj struct {
		EntityType string `json:"entityType"`
		Data       []Task `json:"data"`
	}
	if err := json.Unmarshal(data, &payloadObj); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payloadObj.EntityType != "task" || len(payloadObj.Data) != 1 || payloadObj.Data[0].ID != "t1" || payloadObj.Data[0].Title != "task1" || payloadObj.Data[0].Category != "cat" || payloadObj.Data[0].Order != 1 {
		t.Fatalf("unexpected payload %+v", payloadObj)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SubscribeUpdates did not exit")
	}
}
