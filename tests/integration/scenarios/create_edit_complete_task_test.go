package scenarios

import (
	"fmt"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestCreateEditCompleteTask(t *testing.T) {
	client := newPrismApiClient(t)

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	title := "task-title-" + taskID
	// Create task
	_, err := client.PostJSON("/api/commands", []command{{EntityType: "task", EntityID: taskID, Type: "create-task", Data: map[string]any{"title": title}}}, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Wait for task to appear
	pollTasks(t, client, fmt.Sprintf("task %s to be created with title %s", taskID, title), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == title
			}
		}
		return false
	})

	// Edit task title
	newTitle := title + " updated"
	_, err = client.PostJSON("/api/commands", []command{{EntityType: "task", EntityID: taskID, Type: "update-task", Data: map[string]any{"title": newTitle}}}, nil)
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
	_, err = client.PostJSON("/api/commands", []command{{EntityType: "task", EntityID: taskID, Type: "complete-task"}}, nil)
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}
	tasks := pollTasks(t, client, fmt.Sprintf("task %s to be completed with title %s", taskID, newTitle), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Done && tk.Title == newTitle
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
	assertx.Equal(t, true, final.Done)
}
