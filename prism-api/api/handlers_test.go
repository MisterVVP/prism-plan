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
	log "github.com/sirupsen/logrus"

	"prism-api/domain"
)

type mockStore struct {
	tasks     []domain.Task
	cmds      []domain.Command
	settings  domain.Settings
	nextToken string
	err       error
	lastToken string
	lastLimit int
}

func (m *mockStore) FetchTasks(ctx context.Context, userID, token string, limit int) ([]domain.Task, string, error) {
	m.lastToken = token
	m.lastLimit = limit
	return m.tasks, m.nextToken, m.err
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

type noopStore struct{}

func (noopStore) FetchTasks(context.Context, string, string, int) ([]domain.Task, string, error) {
	return nil, "", nil
}

func (noopStore) FetchSettings(context.Context, string) (domain.Settings, error) {
	return domain.Settings{}, nil
}

func (noopStore) EnqueueCommands(context.Context, string, []domain.Command) error { return nil }

type noopDeduper struct{}

func (noopDeduper) Add(context.Context, string, string) (bool, error) { return true, nil }

func (noopDeduper) Remove(context.Context, string, string) error { return nil }

func resetCommandSenderForTests() {
	shutdownCommandSender()
	globalStore = noopStore{}
	globalDeduper = noopDeduper{}
}

func TestGetTasks(t *testing.T) {
	e := echo.New()
	store := &mockStore{tasks: []domain.Task{{ID: "1", Title: "t"}}, nextToken: "next-token"}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks?pageToken=tok", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := getTasks(store, mockAuth{}, log.New())(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", rec.Code)
	}
	if store.lastToken != "tok" {
		t.Fatalf("expected token to be forwarded, got %q", store.lastToken)
	}
	if store.lastLimit != 0 {
		t.Fatalf("expected default page size when none provided, got %d", store.lastLimit)
	}
	var resp tasksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.Tasks) != 1 || resp.Tasks[0].ID != "1" {
		t.Fatalf("unexpected tasks: %#v", resp.Tasks)
	}
	if resp.NextPageToken != "next-token" {
		t.Fatalf("unexpected next token: %#v", resp.NextPageToken)
	}
}

func TestGetTasksPageSizeProvided(t *testing.T) {
	e := echo.New()
	store := &mockStore{tasks: []domain.Task{{ID: "1", Title: "t"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks?pageSize=120", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := getTasks(store, mockAuth{}, log.New())(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 got %d", rec.Code)
	}
	if store.lastLimit != 120 {
		t.Fatalf("expected page size to be forwarded, got %d", store.lastLimit)
	}
}

func TestGetTasksInvalidPageSize(t *testing.T) {
	testCases := map[string]string{
		"non_numeric": "/api/tasks?pageSize=abc",
		"negative":    "/api/tasks?pageSize=-5",
		"zero":        "/api/tasks?pageSize=0",
	}
	for name, target := range testCases {
		t.Run(name, func(t *testing.T) {
			e := echo.New()
			store := &mockStore{}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := getTasks(store, mockAuth{}, log.New())(c); err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 got %d", rec.Code)
			}
			if store.lastLimit != 0 {
				t.Fatalf("expected store to not be called with invalid page size, got limit %d", store.lastLimit)
			}
		})
	}
}

type invalidTokenErr struct{}

func (invalidTokenErr) Error() string             { return "invalid" }
func (invalidTokenErr) InvalidContinuationToken() {}

func TestGetTasksInvalidToken(t *testing.T) {
	e := echo.New()
	store := &mockStore{err: invalidTokenErr{}}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks?pageToken=bad", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := getTasks(store, mockAuth{}, log.New())(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 got %d", rec.Code)
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

type blockingStore struct {
	mockStore
	calls   chan struct{}
	unblock chan struct{}
	done    chan struct{}
}

func newBlockingStore() *blockingStore {
	return &blockingStore{
		calls:   make(chan struct{}, 16),
		unblock: make(chan struct{}, 16),
		done:    make(chan struct{}, 16),
	}
}

func (b *blockingStore) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	b.calls <- struct{}{}
	<-b.unblock
	if err := b.mockStore.EnqueueCommands(ctx, userID, cmds); err != nil {
		return err
	}
	b.done <- struct{}{}
	return nil
}

func (b *blockingStore) waitForCalls(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-b.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for enqueue call %d", i+1)
		}
	}
}

func (b *blockingStore) waitForDone(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-b.done:
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for enqueue completion %d", i+1)
		}
	}
}

func TestPostCommandsIdempotency(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	logger := log.New()
	deduper, cleanup := setupDeduper(t)
	defer cleanup()
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, deduper, logger)
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
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(store.cmds) == 1 {
			break
		}
		time.Sleep(1 * time.Millisecond)
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
	logger := log.New()
	deduper, cleanup := setupDeduper(t)
	defer cleanup()
	e := echo.New()
	store := &errStore{fail: true}
	handler := postCommands(store, mockAuth{}, deduper, logger)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatalf("post: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 got %d", rec.Code)
	}
	var resp struct {
		IdempotencyKeys []string `json:"idempotencyKeys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.IdempotencyKeys) != 1 || resp.IdempotencyKeys[0] != "k1" {
		t.Fatalf("unexpected idempotency keys: %#v", resp)
	}
}

func TestPostCommandsReturnKeysForAll(t *testing.T) {
	logger := log.New()
	deduper, cleanup := setupDeduper(t)
	defer cleanup()
	// pre-add a key to simulate duplicate
	if _, err := deduper.Add(context.Background(), "user", "k1"); err != nil {
		t.Fatalf("seed deduper: %v", err)
	}
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, deduper, logger)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"},{"entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := handler(c); err != nil {
		t.Fatalf("post: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 got %d", rec.Code)
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

type batchDeduperStub struct {
	t        *testing.T
	results  []bool
	err      error
	rollback []int
	removed  []string
	lastKeys []string
}

func (b *batchDeduperStub) Add(ctx context.Context, userID, key string) (bool, error) {
	b.t.Fatalf("unexpected Add call for key %s", key)
	return false, nil
}

func (b *batchDeduperStub) Remove(ctx context.Context, userID, key string) error {
	b.removed = append(b.removed, key)
	return nil
}

func (b *batchDeduperStub) AddMany(ctx context.Context, userID string, keys []string) ([]bool, error) {
	b.lastKeys = append([]string(nil), keys...)
	if len(b.results) != len(keys) {
		b.t.Fatalf("unexpected keys length: got %d, want %d", len(keys), len(b.results))
	}
	if b.err == nil {
		return append([]bool(nil), b.results...), nil
	}
	if len(b.rollback) == 0 {
		return append([]bool(nil), b.results...), b.err
	}
	return append([]bool(nil), b.results...), &batchRollbackError{err: b.err, idx: append([]int(nil), b.rollback...)}
}

type batchRollbackError struct {
	err error
	idx []int
}

func (e *batchRollbackError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *batchRollbackError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *batchRollbackError) RollbackIndexes() []int {
	if e == nil {
		return nil
	}
	out := make([]int, len(e.idx))
	copy(out, e.idx)
	return out
}

func TestPostCommandsCleansUpOnDeduperError(t *testing.T) {
	logger := log.New()
	d := newFlakeyDeduper(1)
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, d, logger)
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
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 got %d", rec2.Code)
	}
	if len(d.keys) != 2 {
		t.Fatalf("expected 2 keys added, got %d", len(d.keys))
	}
}

func TestPostCommandsFallbackWhenQueueFull(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	t.Setenv("ENQUEUE_BUFFER", "1")
	t.Setenv("ENQUEUE_WORKERS", "1")
	t.Setenv("ENQUEUE_TIMEOUT", "1s")
	t.Setenv("ENQUEUE_HANDOFF_TIMEOUT", "0s")

	logger := log.New()
	deduper, cleanup := setupDeduper(t)
	defer cleanup()

	store := newBlockingStore()
	handler := postCommands(store, mockAuth{}, deduper, logger)

	e := echo.New()
	body := `[{"entityType":"task","type":"create-task"}]`

	req1 := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req1.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec1 := httptest.NewRecorder()
	if err := handler(e.NewContext(req1, rec1)); err != nil {
		t.Fatalf("first post: %v", err)
	}
	if rec1.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 got %d", rec1.Code)
	}
	store.waitForCalls(t, 1)

	req2 := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req2.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	if err := handler(e.NewContext(req2, rec2)); err != nil {
		t.Fatalf("second post: %v", err)
	}
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 got %d", rec2.Code)
	}

	req3 := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req3.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req3.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec3 := httptest.NewRecorder()
	done := make(chan error, 1)
	go func() {
		done <- handler(e.NewContext(req3, rec3))
	}()

	store.waitForCalls(t, 1) // inline fallback attempted

	store.unblock <- struct{}{}
	store.waitForDone(t, 1)

	store.waitForCalls(t, 1) // worker picked queued job

	store.unblock <- struct{}{}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("third post: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for third post completion")
	}

	if rec3.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 got %d", rec3.Code)
	}

	store.waitForDone(t, 1)

	store.unblock <- struct{}{}
	store.waitForDone(t, 1)

	if len(store.cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(store.cmds))
	}
}

func TestPostCommandsUsesBatchDeduper(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	logger := log.New()
	deduper := &batchDeduperStub{t: t, results: []bool{true, false, true}}
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, deduper, logger)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"},{"idempotencyKey":"k2","entityType":"task","type":"create-task"},{"entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("post: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 got %d", rec.Code)
	}
	if len(deduper.lastKeys) != 3 {
		t.Fatalf("expected 3 keys passed to AddMany, got %d", len(deduper.lastKeys))
	}

	var resp struct {
		IdempotencyKeys []string `json:"idempotencyKeys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.IdempotencyKeys) != 3 {
		t.Fatalf("expected 3 idempotency keys, got %d", len(resp.IdempotencyKeys))
	}

	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(store.cmds) == 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(store.cmds) != 2 {
		t.Fatalf("expected 2 commands enqueued, got %d", len(store.cmds))
	}
	if store.cmds[0].ID != deduper.lastKeys[0] {
		t.Fatalf("unexpected command id %s", store.cmds[0].ID)
	}
	if store.cmds[1].ID != deduper.lastKeys[2] {
		t.Fatalf("unexpected second command id %s", store.cmds[1].ID)
	}
	if len(deduper.removed) != 0 {
		t.Fatalf("expected no removals, got %v", deduper.removed)
	}
}

func TestPostCommandsBatchDeduperError(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	logger := log.New()
	deduper := &batchDeduperStub{t: t, results: []bool{true, false}, err: errors.New("batch failure"), rollback: []int{1}}
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, deduper, logger)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"},{"idempotencyKey":"k2","entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("post: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 got %d", rec.Code)
	}
	if len(deduper.removed) != 2 {
		t.Fatalf("expected removal of both keys, got %v", deduper.removed)
	}
	removed := map[string]int{}
	for _, key := range deduper.removed {
		removed[key]++
	}
	if removed["k1"] != 1 || removed["k2"] != 1 {
		t.Fatalf("expected single removal of k1 and k2, got %v", deduper.removed)
	}
	if len(store.cmds) != 0 {
		t.Fatalf("expected no commands enqueued, got %d", len(store.cmds))
	}
}

func TestPostCommandsBatchDeduperErrorRemovesFailedKey(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	logger := log.New()
	deduper := &batchDeduperStub{t: t, results: []bool{false, false}, err: errors.New("batch failure"), rollback: []int{1}}
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{}, deduper, logger)
	body := `[{"idempotencyKey":"k1","entityType":"task","type":"create-task"},{"idempotencyKey":"k2","entityType":"task","type":"create-task"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("post: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 got %d", rec.Code)
	}
	t.Logf("removed keys: %v", deduper.removed)
	if len(deduper.removed) != 1 || deduper.removed[0] != "k2" {
		t.Fatalf("expected removal of only k2, got %v", deduper.removed)
	}
	if len(store.cmds) != 0 {
		t.Fatalf("expected no commands enqueued, got %d", len(store.cmds))
	}
}
