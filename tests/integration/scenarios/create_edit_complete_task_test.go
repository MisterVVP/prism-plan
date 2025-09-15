package scenarios

import (
	"fmt"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestCreateEditCompleteTask(t *testing.T) {
	client := newPrismApiClient(t)

	title := fmt.Sprintf("task-title-%d", time.Now().UnixNano())
	// Create task
	_, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-create-%s", title), EntityType: "task", Type: "create-task", Data: map[string]any{"title": title}}}, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Wait for task to appear
	var taskID string
	pollTasks(t, client, fmt.Sprintf("task with title %s to be created", title), func(ts []task) bool {
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
	_, err = client.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-update-%s", taskID), EntityType: "task", Type: "update-task", Data: map[string]any{"id": taskID, "title": newTitle}}}, nil)
	if err != nil {
		t.Fatalf("edit task: %v", err)
	}
	pollTasks(t, client, fmt.Sprintf("task %s to have updated title %s", taskID, newTitle), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == newTitle
			}
		}
		return false
	})

	// Complete task
	_, err = client.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-complete-%s", taskID), EntityType: "task", Type: "complete-task", Data: map[string]any{"id": taskID}}}, nil)
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}
	pollTasks(t, client, fmt.Sprintf("task %s to be completed with title %s", taskID, newTitle), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Done && tk.Title == newTitle
			}
		}
		return false
	})

	// Reopen task
	_, err = client.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-reopen-%s", taskID), EntityType: "task", Type: "reopen-task", Data: map[string]any{"id": taskID}}}, nil)
	if err != nil {
		t.Fatalf("reopen task: %v", err)
	}
	tasks := pollTasks(t, client, fmt.Sprintf("task %s to be reopened", taskID), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return !tk.Done
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
	assertx.Equal(t, false, final.Done)
}
