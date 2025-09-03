package scenarios

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestStaleUpdateMergesFields(t *testing.T) {
	rmuClient := newReadModelUpdaterClient(t)
	apiClient := newPrismApiClient(t)

	taskID := fmt.Sprintf("stale-%d", time.Now().UnixNano())
	userID := "integration-user"

	send := func(ev map[string]any) {
		b, _ := json.Marshal(ev)
		payload := map[string]any{"Data": map[string]any{"event": string(b)}}
		resp, err := rmuClient.PostJSON("/domain-events", payload, nil)
		if err != nil {
			t.Fatalf("post event: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected status %d", resp.StatusCode)
		}
	}

	send(map[string]any{
		"Id":         "1",
		"EntityId":   taskID,
		"EntityType": "task",
		"Type":       "task-created",
		"Timestamp":  1,
		"UserId":     userID,
		"Data":       map[string]any{"title": "t"},
	})
	send(map[string]any{
		"Id":         "2",
		"EntityId":   taskID,
		"EntityType": "task",
		"Type":       "task-updated",
		"Timestamp":  3,
		"UserId":     userID,
		"Data":       map[string]any{"done": true},
	})
	send(map[string]any{
		"Id":         "3",
		"EntityId":   taskID,
		"EntityType": "task",
		"Type":       "task-updated",
		"Timestamp":  2,
		"UserId":     userID,
		"Data":       map[string]any{"notes": "note"},
	})

	pollTasks(t, apiClient, fmt.Sprintf("task %s to have merged notes", taskID), func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == taskID {
				return tk.Done && tk.Notes == "note"
			}
		}
		return false
	})
}
