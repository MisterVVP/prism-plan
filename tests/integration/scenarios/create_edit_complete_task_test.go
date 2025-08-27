package scenarios

import (
	"fmt"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestCreateEditCompleteTask(t *testing.T) {
	client := newClient(t)

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	title := "task-title-" + taskID
	// Create task
	_, err := client.PostJSON("/api/commands", []command{{EntityType: "task", EntityID: taskID, Type: "create-task", Data: map[string]interface{}{"title": title}}}, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Wait for task to appear
	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == title
			}
		}
		return false
	})

	// Edit task title
	newTitle := title + " updated"
	_, err = client.PostJSON("/api/commands", []command{{EntityType: "task", EntityID: taskID, Type: "update-task", Data: map[string]interface{}{"title": newTitle}}}, nil)
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
	_, err = client.PostJSON("/api/commands", []command{{EntityType: "task", EntityID: taskID, Type: "complete-task"}}, nil)
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
