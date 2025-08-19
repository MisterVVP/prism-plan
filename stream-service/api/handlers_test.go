package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

type fakeAuth struct{}

func (fakeAuth) UserIDFromAuthHeader(string) (string, error) { return "user1", nil }

type flushRecorder struct{ *httptest.ResponseRecorder }

func (flushRecorder) Flush() {}

func setupRedis(t *testing.T) (*redis.Client, func()) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	return rc, func() {
		rc.Close()
		m.Close()
	}
}

func TestAddRemoveClientBroadcast(t *testing.T) {
	clients = map[string]map[chan []byte]struct{}{}
	ch := make(chan []byte, 1)
	addClient("user1", ch)
	broadcast("user1", []byte("hello"))
	select {
	case msg := <-ch:
		if string(msg) != "hello" {
			t.Fatalf("expected hello got %s", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("no message received")
	}
	removeClient("user1", ch)
	broadcast("user1", []byte("world"))
	select {
	case <-ch:
		t.Fatal("received message after removal")
	default:
	}
}

func TestStreamTasksFetchesFromStoreAndCaches(t *testing.T) {
	rc, cleanup := setupRedis(t)
	defer cleanup()
	store := &fakeStore{tasks: []domain.Task{{ID: "1", Title: "t"}}}
	auth := fakeAuth{}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := flushRecorder{httptest.NewRecorder()}
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	c := e.NewContext(req, rec)
	handler := streamTasks(store, rc, auth)

	errCh := make(chan error, 1)
	go func() { errCh <- handler(c) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("handler error: %v", err)
	}

	expectedData, _ := json.Marshal(store.tasks)
	expected := consts.SSEDataPrefix + string(expectedData) + "\n\n"
	if rec.Body.String() != expected {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
	if store.called != 1 {
		t.Fatalf("expected FetchTasks once, got %d", store.called)
	}
	if val := rc.Get(context.Background(), consts.TasksKeyPrefix+"user1").Val(); val != string(expectedData) {
		t.Fatalf("expected cache %s, got %s", string(expectedData), val)
	}
}

func TestStreamTasksUsesCache(t *testing.T) {
	rc, cleanup := setupRedis(t)
	defer cleanup()
	tasks := []domain.Task{{ID: "1", Title: "cached"}}
	data, _ := json.Marshal(tasks)
	if err := rc.Set(context.Background(), consts.TasksKeyPrefix+"user1", data, 0).Err(); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	store := &fakeStore{tasks: tasks}
	auth := fakeAuth{}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := flushRecorder{httptest.NewRecorder()}
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	c := e.NewContext(req, rec)
	handler := streamTasks(store, rc, auth)

	errCh := make(chan error, 1)
	go func() { errCh <- handler(c) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("handler error: %v", err)
	}

	expected := consts.SSEDataPrefix + string(data) + "\n\n"
	if rec.Body.String() != expected {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
	if store.called != 0 {
		t.Fatalf("expected no store calls, got %d", store.called)
	}
}
