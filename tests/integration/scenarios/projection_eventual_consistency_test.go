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

	taskID := fmt.Sprintf("consistency-%d", time.Now().UnixNano())
	title := "consistency-title-" + taskID
	start := time.Now()
	resp, err := client.PostJSON("/api/commands", []command{{EntityType: "task", EntityID: taskID, Type: "create-task", Data: map[string]any{"title": title}}}, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Title == title
			}
		}
		return false
	})
	dur := time.Since(start)
	if dur > timeout {
		t.Fatalf("projection took %v, exceeds timeout %v", dur, timeout)
	}
}
