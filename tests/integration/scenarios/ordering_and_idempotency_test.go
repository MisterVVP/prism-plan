package scenarios

import (
	"fmt"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestOrderingAndIdempotency(t *testing.T) {
	client := newPrismApiClient(t)

	title := fmt.Sprintf("title-%d", time.Now().UnixNano())
	dedupe := fmt.Sprintf("ik-create-%d", time.Now().UnixNano())
	payload := map[string]any{"title": title}

	if resp, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: dedupe, EntityType: "task", Type: "create-task", Data: payload}}, nil); err != nil || resp.StatusCode >= 300 {
		t.Fatalf("first create: status %d err %v", resp.StatusCode, err)
	}
	if resp, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: dedupe, EntityType: "task", Type: "create-task", Data: payload}}, nil); err != nil || resp.StatusCode >= 300 {
		t.Fatalf("second create: status %d err %v", resp.StatusCode, err)
	}

	var taskID string
	tasks := pollTasks(t, client, fmt.Sprintf("task with title %s to be created", title), func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				taskID = tk.ID
				return true
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

	completeKey := fmt.Sprintf("ik-complete-%d", time.Now().UnixNano())
	completePayload := map[string]any{"id": taskID}
	for i := 0; i < 2; i++ {
		if resp, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: completeKey, EntityType: "task", Type: "complete-task", Data: completePayload}}, nil); err != nil || resp.StatusCode >= 300 {
			t.Fatalf("complete %d: status %d err %v", i, resp.StatusCode, err)
		}
	}

	pollTasks(t, client, fmt.Sprintf("task %s to be marked done", taskID), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Done
			}
		}
		return false
	})

	reopenKey := fmt.Sprintf("ik-reopen-%d", time.Now().UnixNano())
	reopenPayload := map[string]any{"id": taskID}
	for i := 0; i < 2; i++ {
		if resp, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: reopenKey, EntityType: "task", Type: "reopen-task", Data: reopenPayload}}, nil); err != nil || resp.StatusCode >= 300 {
			t.Fatalf("reopen %d: status %d err %v", i, resp.StatusCode, err)
		}
	}

	pollTasks(t, client, fmt.Sprintf("task %s to be reopened", taskID), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return !tk.Done
			}
		}
		return false
	})

	titles := []string{title + "-a", title + "-b", title + "-c"}
	for i, tt := range titles {
		key := fmt.Sprintf("ik-update-%d", i)
		if resp, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: key, EntityType: "task", Type: "update-task", Data: map[string]any{"id": taskID, "title": tt}}}, nil); err != nil || resp.StatusCode >= 300 {
			t.Fatalf("edit %d: status %d err %v", i, resp.StatusCode, err)
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
