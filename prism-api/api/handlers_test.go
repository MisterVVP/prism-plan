package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"prism-api/domain"
)

type mockStore struct {
	tasks []domain.Task
	cmds  []domain.Command
}

func (m *mockStore) FetchTasks(ctx context.Context, userID string) ([]domain.Task, error) {
	return m.tasks, nil
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
