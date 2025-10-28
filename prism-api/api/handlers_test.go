package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"

	"prism-api/domain"
)

type mockStore struct {
	tasks     []domain.Task
	settings  domain.Settings
	nextToken string
	err       error
	lastToken string
	lastLimit int

	mu   sync.Mutex
	cmds []domain.Command
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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cmds = append(m.cmds, cmds...)
	return nil
}

func (m *mockStore) Commands() []domain.Command {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Command, len(m.cmds))
	copy(out, m.cmds)
	return out
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

func resetCommandSenderForTests() {
	shutdownCommandSender()
	globalStore = noopStore{}
}

func TestFinalizeCommandsSequentialTimestamps(t *testing.T) {
	t.Cleanup(func() {
		atomic.StoreInt64(&lastTimestamp, 0)
	})
	atomic.StoreInt64(&lastTimestamp, time.Now().Add(time.Second).UnixNano())

	cmds := []domain.Command{{EntityType: "task", Type: "create"}, {IdempotencyKey: "known", EntityType: "task", Type: "update"}}
	keys := finalizeCommands(cmds)

	if len(keys) != len(cmds) {
		t.Fatalf("expected %d keys, got %d", len(cmds), len(keys))
	}
	if keys[1] != "known" {
		t.Fatalf("expected existing key to be preserved, got %q", keys[1])
	}

	firstTS := cmds[0].Timestamp
	secondTS := cmds[1].Timestamp
	if secondTS-firstTS != 1 {
		t.Fatalf("expected timestamps to increment by 1, got first=%d second=%d", firstTS, secondTS)
	}

	expectedKey := strconv.FormatInt(firstTS, 36)
	if keys[0] != expectedKey {
		t.Fatalf("expected generated key %q, got %q", expectedKey, keys[0])
	}
	if cmds[0].ID != expectedKey {
		t.Fatalf("expected command ID %q, got %q", expectedKey, cmds[0].ID)
	}
	if cmds[1].ID != "known" {
		t.Fatalf("expected command ID 'known', got %q", cmds[1].ID)
	}
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
	if err := sonic.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
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
	if err := sonic.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if s.TasksPerCategory != 3 || !s.ShowDoneTasks {
		t.Fatalf("unexpected settings: %#v", s)
	}
}

func waitForCommands(t *testing.T, store *mockStore, expected int) []domain.Command {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	for {
		cmds := store.Commands()
		if len(cmds) == expected {
			return cmds
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %d commands, got %d", expected, len(cmds))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPostCommandsEnqueuesCommandsAndReturnsKeys(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	e := echo.New()
	store := &mockStore{}
	initCommandSender(store, log.New())
	handler := postCommands(store, mockAuth{})

	body := `[{"entityType":"task","type":"create-task"},{"idempotencyKey":"known","entityType":"task","type":"update-task"}]`
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
	if err := sonic.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.IdempotencyKeys) != 2 {
		t.Fatalf("expected 2 idempotency keys, got %d", len(resp.IdempotencyKeys))
	}
	if resp.IdempotencyKeys[0] == "" {
		t.Fatalf("expected generated key for first command")
	}
	if resp.IdempotencyKeys[1] != "known" {
		t.Fatalf("expected to echo provided key, got %q", resp.IdempotencyKeys[1])
	}

	cmds := waitForCommands(t, store, 2)
	if cmds[0].ID != resp.IdempotencyKeys[0] {
		t.Fatalf("expected first command ID %q, got %q", resp.IdempotencyKeys[0], cmds[0].ID)
	}
	if cmds[1].ID != "known" {
		t.Fatalf("expected second command ID 'known', got %q", cmds[1].ID)
	}
}

func TestPostCommandsInlineFallbackSuccess(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{})

	body := `[{"entityType":"task","type":"create-task"}]`
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
	if err := sonic.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.IdempotencyKeys) != 1 || resp.IdempotencyKeys[0] == "" {
		t.Fatalf("expected single generated key, got %#v", resp.IdempotencyKeys)
	}
	cmds := store.Commands()
	if len(cmds) != 1 {
		t.Fatalf("expected inline enqueue to run immediately, got %d commands", len(cmds))
	}
	if cmds[0].ID != resp.IdempotencyKeys[0] {
		t.Fatalf("expected command ID %q, got %q", resp.IdempotencyKeys[0], cmds[0].ID)
	}
}

type failingStore struct {
	mockStore
}

func (f *failingStore) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	return errors.New("enqueue failed")
}

func TestPostCommandsInlineFailure(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	e := echo.New()
	store := &failingStore{}
	handler := postCommands(store, mockAuth{})

	body := `[{"entityType":"task","type":"create-task"}]`
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
	if cmds := store.Commands(); len(cmds) != 0 {
		t.Fatalf("expected no commands recorded on failure, got %d", len(cmds))
	}
}
