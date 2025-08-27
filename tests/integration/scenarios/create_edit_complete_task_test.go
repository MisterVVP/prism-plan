package scenarios

import (
	"fmt"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestCreateEditCompleteTask(t *testing.T) {
	client := newClient(t)

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
