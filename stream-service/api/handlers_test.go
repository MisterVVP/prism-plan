package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"stream-service/domain"
)

type mockStore struct {
	tasks []domain.Task
}

func (m *mockStore) FetchTasks(ctx context.Context, userID string) ([]domain.Task, error) {
	return m.tasks, nil
}

type mockAuth struct{}

func (mockAuth) UserIDFromAuthHeader(string) (string, error) { return "user", nil }

func TestUpdateBroadcast(t *testing.T) {
	e := echo.New()
	store := &mockStore{tasks: []domain.Task{{ID: "1"}}}
	broker := newUpdateBroker("secret")
	e.GET("/stream", streamTasks(store, mockAuth{}, broker))
	e.POST("/updates", broker.handleUpdate)

	server := httptest.NewServer(e)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/stream", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	readTasks := func() []domain.Task {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read line: %v", err)
		}
		data := bytes.TrimPrefix([]byte(line), []byte("data: "))
		// consume empty line
		if _, err := reader.ReadString('\n'); err != nil {
			t.Fatalf("read newline: %v", err)
		}
		var tasks []domain.Task
		if err := json.Unmarshal(bytes.TrimSpace(data), &tasks); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return tasks
	}

	tasks := readTasks()
	if len(tasks) != 1 || tasks[0].ID != "1" {
		t.Fatalf("unexpected initial tasks: %#v", tasks)
	}

	store.tasks = []domain.Task{{ID: "2"}}
	body, _ := json.Marshal(map[string]string{"userID": "u", "taskID": "2"})
	req, _ = http.NewRequest(http.MethodPost, server.URL+"/updates", bytes.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer secret")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if _, err := http.DefaultClient.Do(req); err != nil {
		t.Fatalf("post update: %v", err)
	}

	tasks = readTasks()
	if len(tasks) != 1 || tasks[0].ID != "2" {
		t.Fatalf("unexpected updated tasks: %#v", tasks)
	}
}
