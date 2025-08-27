package scenarios

import (
	"fmt"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestOrderingAndIdempotency(t *testing.T) {
	client := newClient(t)

	title := fmt.Sprintf("idempotent-%d", time.Now().UnixNano())
	dedupe := fmt.Sprintf("dk-%d", time.Now().UnixNano())
	payload := map[string]interface{}{"title": title, "dedupeKey": dedupe}

	if _, err := client.PostJSON("/api/commands", command{Type: "CreateTask", Payload: payload}, nil); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := client.PostJSON("/api/commands", command{Type: "CreateTask", Payload: payload}, nil); err != nil {
		t.Fatalf("second create: %v", err)
	}

	tasks := pollTasks(t, client, func(ts []task) bool {
		count := 0
		for _, tk := range ts {
			if tk.Title == title {
				count++
			}
		}
		return count > 0
	})

	count := 0
	var taskID string
	for _, tk := range tasks {
		if tk.Title == title {
			count++
			taskID = tk.ID
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 task, got %d", count)
	}

	titles := []string{title + "-a", title + "-b", title + "-c"}
	for _, tt := range titles {
		if _, err := client.PostJSON("/api/commands", command{Type: "EditTask", Payload: map[string]interface{}{"id": taskID, "title": tt}}, nil); err != nil {
			t.Fatalf("edit: %v", err)
		}
	}
	finalTitle := titles[len(titles)-1]
	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == finalTitle
			}
		}
		return false
	})
	tasks = pollTasks(t, client, func(ts []task) bool {
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
