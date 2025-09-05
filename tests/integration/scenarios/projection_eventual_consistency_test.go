package scenarios

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestProjectionEventualConsistency(t *testing.T) {
	client := newPrismApiClient(t)
	timeout := getPollTimeout(t)

	title := fmt.Sprintf("consistency-title-%d", time.Now().UnixNano())
	start := time.Now()
	resp, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-create-%s", title), EntityType: "task", Type: "create-task", Data: map[string]any{"title": title}}}, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	pollTasks(t, client, fmt.Sprintf("task with title %s to be created", title), func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				return true
			}
		}
		return false
	})
	dur := time.Since(start)
	if dur > timeout {
		t.Fatalf("projection took %v, exceeds timeout %v", dur, timeout)
	}
}
