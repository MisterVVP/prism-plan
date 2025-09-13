package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"prism-api/domain"
)

type mockStore struct {
	tasks    []domain.Task
	cmds     []domain.Command
	settings domain.Settings
}

func (m *mockStore) FetchTasks(ctx context.Context, userID string) ([]domain.Task, error) {
	return m.tasks, nil
}

func (m *mockStore) FetchSettings(ctx context.Context, userID string) (domain.Settings, error) {
	return m.settings, nil
}

func (m *mockStore) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	m.cmds = append(m.cmds, cmds...)
	return nil
}

type mockAuth struct{}

func (mockAuth) UserIDFromAuthHeader(string) (string, error) { return "user", nil }

func TestGetTasks(t *testing.T) {
	e := echo.New()
	store := &mockStore{tasks: []domain.Task{{ID: "1", Title: "t"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := getTasks(store, mockAuth{})(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", rec.Code)
	}
	var tasks []domain.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &tasks); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "1" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestGetSettings(t *testing.T) {
	e := echo.New()
	store := &mockStore{settings: domain.Settings{TasksPerCategory: 3, ShowDoneTasks: true}}
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := getSettings(store, mockAuth{})(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", rec.Code)
	}
	var s domain.Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if s.TasksPerCategory != 3 || !s.ShowDoneTasks {
		t.Fatalf("unexpected settings: %#v", s)
	}
}

func setupDeduper(t *testing.T) (Deduper, func()) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	rc := redis.NewClient(&redis.Options{Addr: m.Addr()})
	d := NewRedisDeduper(rc, time.Hour)
	return d, func() {
		if err := rc.Close(); err != nil {
			t.Logf("redis close: %v", err)
		}
		m.Close()
	}
}

func TestPostCommandsIdempotency(t *testing.T) {
	deduper, cleanup := setupDeduper(t)
	defer cleanup()
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, deduper)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatalf("first post: %v", err)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req2.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	if err := handler(c2); err != nil {
		t.Fatalf("second post: %v", err)
	}
	if len(store.cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(store.cmds))
	}
	if store.cmds[0].ID != "k1" {
		t.Fatalf("command ID %s does not match expected idempotency key", store.cmds[0].ID)
	}
	var resp struct {
		IdempotencyKeys []string `json:"idempotencyKeys"`
		Error           string   `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.IdempotencyKeys) != 1 || resp.IdempotencyKeys[0] != "k1" {
		t.Fatalf("unexpected idempotency keys: %#v", resp)
	}
	var resp2 map[string][]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp2["idempotencyKeys"]) != 1 || resp2["idempotencyKeys"][0] != "k1" {
		t.Fatalf("unexpected idempotency keys on second post: %#v", resp2)
	}
}

type errStore struct {
	mockStore
	fail bool
}

func (e *errStore) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	if e.fail {
		return errors.New("enqueue failed")
	}
	return e.mockStore.EnqueueCommands(ctx, userID, cmds)
}

func TestPostCommandsRetryOnError(t *testing.T) {
	deduper, cleanup := setupDeduper(t)
	defer cleanup()
	e := echo.New()
	store := &errStore{fail: true}
	handler := postCommands(store, mockAuth{}, deduper)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatalf("first post: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 got %d", rec.Code)
	}
	if len(store.cmds) != 0 {
		t.Fatalf("expected no commands, got %d", len(store.cmds))
	}
	var resp struct {
		IdempotencyKeys []string `json:"idempotencyKeys"`
		Error           string   `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.IdempotencyKeys) != 1 || resp.IdempotencyKeys[0] != "k1" {
		t.Fatalf("unexpected idempotency keys: %#v", resp)
	}
	store.fail = false
	req2 := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req2.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	if err := handler(c2); err != nil {
		t.Fatalf("retry post: %v", err)
	}
	if len(store.cmds) != 1 {
		t.Fatalf("expected 1 command after retry, got %d", len(store.cmds))
	}
	var resp2 map[string][]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp2["idempotencyKeys"]) != 1 || resp2["idempotencyKeys"][0] != "k1" {
		t.Fatalf("unexpected idempotency keys after retry: %#v", resp2)
	}
}

func TestPostCommandsReturnKeysForAll(t *testing.T) {
	deduper, cleanup := setupDeduper(t)
	defer cleanup()
	// pre-add a key to simulate duplicate
	if _, err := deduper.Add(context.Background(), "user", "k1"); err != nil {
		t.Fatalf("seed deduper: %v", err)
	}
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, deduper)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"},{"entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatalf("post: %v", err)
	}
	if len(store.cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(store.cmds))
	}
	var resp map[string][]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	keys := resp["idempotencyKeys"]
	if len(keys) != 2 {
		t.Fatalf("expected 2 idempotency keys, got %d", len(keys))
	}
	if keys[0] != "k1" {
		t.Fatalf("expected first key k1, got %s", keys[0])
	}
	if keys[1] == "" || keys[1] == "k1" {
		t.Fatalf("invalid second key: %s", keys[1])
	}
	if store.cmds[0].IdempotencyKey != keys[1] {
		t.Fatalf("stored command key %s does not match response %s", store.cmds[0].IdempotencyKey, keys[1])
	}
	if store.cmds[0].ID != store.cmds[0].IdempotencyKey {
		t.Fatalf("command ID %s does not match idempotency key %s", store.cmds[0].ID, store.cmds[0].IdempotencyKey)
	}
}

type flakeyDeduper struct {
	keys   map[string]struct{}
	failAt int
	calls  int
}

func newFlakeyDeduper(failAt int) *flakeyDeduper {
	return &flakeyDeduper{keys: make(map[string]struct{}), failAt: failAt}
}

func (d *flakeyDeduper) Add(ctx context.Context, userID, key string) (bool, error) {
	if d.calls == d.failAt {
		d.calls++
		return false, errors.New("add failed")
	}
	d.calls++
	if _, ok := d.keys[key]; ok {
		return false, nil
	}
	d.keys[key] = struct{}{}
	return true, nil
}

func (d *flakeyDeduper) Remove(ctx context.Context, userID, key string) error {
	delete(d.keys, key)
	return nil
}

func TestPostCommandsCleansUpOnDeduperError(t *testing.T) {
	d := newFlakeyDeduper(1)
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, d)
	body := `[{"entityType":"task","type":"create-task"},{"entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatalf("first post: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 got %d", rec.Code)
	}
	if len(d.keys) != 0 {
		t.Fatalf("expected keys cleaned up, got %v", d.keys)
	}

	// Allow subsequent calls to succeed
	d.failAt = -1
	req2 := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req2.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	if err := handler(c2); err != nil {
		t.Fatalf("second post: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", rec2.Code)
	}
	if len(d.keys) != 2 {
		t.Fatalf("expected 2 keys added, got %d", len(d.keys))
	}
}
