package scenarios

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	integration "prismtest"
	"prismtest/internal/assertx"
	"prismtest/internal/httpclient"
)

type command struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type task struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// pollTasks polls /api/tasks until cond returns true or timeout.
func pollTasks(t *testing.T, client *httpclient.Client, cond func([]task) bool) []task {
	deadline := time.Now().Add(10 * time.Second)
	backoff := 200 * time.Millisecond
	for {
		var tasks []task
		_, err := client.GetJSON("/api/tasks", &tasks)
		if err == nil && cond(tasks) {
			return tasks
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for tasks: %v", err)
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
}

func TestCreateEditCompleteTask(t *testing.T) {
	base := os.Getenv("API_BASE")
	if base == "" {
		base = "http://localhost"
	}
	if _, err := http.Get(base + "/healthz"); err != nil {
		t.Skipf("skipping, API not reachable: %v", err)
	}
	bearer := os.Getenv("TEST_BEARER")
	if bearer == "" {
		tok, err := integration.TestToken("integration-user")
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		bearer = tok
	}
	client := httpclient.New(base, bearer)

	title := fmt.Sprintf("task-%d", time.Now().UnixNano())
	// Create task
	_, err := client.PostJSON("/api/commands", command{Type: "CreateTask", Payload: map[string]interface{}{"title": title}}, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Wait for task to appear and capture ID
	var taskID string
	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				taskID = tk.ID
				return true
			}
		}
		return false
	})

	// Edit task title
	newTitle := title + " updated"
	_, err = client.PostJSON("/api/commands", command{Type: "EditTask", Payload: map[string]interface{}{"id": taskID, "title": newTitle}}, nil)
	if err != nil {
		t.Fatalf("edit task: %v", err)
	}
	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == newTitle
			}
		}
		return false
	})

	// Complete task
	_, err = client.PostJSON("/api/commands", command{Type: "CompleteTask", Payload: map[string]interface{}{"id": taskID}}, nil)
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}
	tasks := pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Status == "Completed" && tk.Title == newTitle
			}
		}
		return false
	})

	var final task
	for _, tk := range tasks {
		if tk.ID == taskID {
			final = tk
			break
		}
	}
	assertx.Equal(t, newTitle, final.Title)
	assertx.Equal(t, "Completed", final.Status)
}
