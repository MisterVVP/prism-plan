package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

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

func TestPostCommandsIdempotency(t *testing.T) {
	e := echo.New()
	store := &mockStore{}
	handler := postCommands(store, mockAuth{})
	body := `[{
                "id":"",
                "idempotencyKey":"k1",
                "entityId":"",
                "entityType":"task",
                "type":"create-task"
        }]`
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
}
