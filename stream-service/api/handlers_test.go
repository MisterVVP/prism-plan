package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"stream-service/domain"
)

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

func TestStreamTasksReceivesUpdates(t *testing.T) {
	rc, cleanup := setupRedis(t)
	defer cleanup()
	auth := fakeAuth{}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := flushRecorder{httptest.NewRecorder()}
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	c := e.NewContext(req, rec)
	handler := streamTasks(rc, auth)

	errCh := make(chan error, 1)
	go func() { errCh <- handler(c) }()
	time.Sleep(100 * time.Millisecond)
	update := []byte(`{"hello":"world"}`)
	broadcast("user1", update)
	time.Sleep(100 * time.Millisecond)
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("handler error: %v", err)
	}

	expected := domain.SSEDataPrefix + "[]\n\n" + domain.SSEDataPrefix + string(update) + "\n\n"
	if rec.Body.String() != expected {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}

func TestStreamTasksUsesCachedPayload(t *testing.T) {
	rc, cleanup := setupRedis(t)
	defer cleanup()
	payload := []byte(`{"cached":true}`)
	if err := rc.Set(context.Background(), domain.TasksKeyPrefix+"user1", payload, 0).Err(); err != nil {
		t.Fatalf("set cache: %v", err)
	}
	auth := fakeAuth{}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := flushRecorder{httptest.NewRecorder()}
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	c := e.NewContext(req, rec)
	handler := streamTasks(rc, auth)

	errCh := make(chan error, 1)
	go func() { errCh <- handler(c) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("handler error: %v", err)
	}

	expected := domain.SSEDataPrefix + string(payload) + "\n\n"
	if rec.Body.String() != expected {
		t.Fatalf("unexpected body %q", rec.Body.String())
	}
}
