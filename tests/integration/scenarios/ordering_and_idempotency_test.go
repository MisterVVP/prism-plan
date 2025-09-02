package scenarios

import (
	"fmt"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestOrderingAndIdempotency(t *testing.T) {
	client := newPrismApiClient(t)

	taskID := fmt.Sprintf("idempotent-%d", time.Now().UnixNano())
	title := fmt.Sprintf("title-%d", time.Now().UnixNano())
	dedupe := fmt.Sprintf("ik-create-%d", time.Now().UnixNano())
	payload := map[string]any{"title": title}

	if _, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: dedupe, EntityType: "task", EntityID: taskID, Type: "create-task", Data: payload}}, nil); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: dedupe, EntityType: "task", EntityID: taskID, Type: "create-task", Data: payload}}, nil); err != nil {
		t.Fatalf("second create: %v", err)
	}

	tasks := pollTasks(t, client, fmt.Sprintf("task %s to be created with title %s", taskID, title), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == title
			}
		}
		return false
	})

	count := 0
	for _, tk := range tasks {
		if tk.ID == taskID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 task, got %d", count)
	}

	titles := []string{title + "-a", title + "-b", title + "-c"}
	for i, tt := range titles {
		key := fmt.Sprintf("ik-update-%d", i)
		if _, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: key, EntityType: "task", EntityID: taskID, Type: "update-task", Data: map[string]any{"title": tt}}}, nil); err != nil {
			t.Fatalf("edit: %v", err)
		}
	}
	finalTitle := titles[len(titles)-1]
	tasks = pollTasks(t, client, fmt.Sprintf("task %s to have final title %s", taskID, finalTitle), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == finalTitle
			}
		}
		return false
	})
	for _, tk := range tasks {
		if tk.ID == taskID {
			assertx.Equal(t, finalTitle, tk.Title)
		}
	}
}
