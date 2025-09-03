package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
)

func TestStaleUpdateMergesFields(t *testing.T) {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING_LOCAL")
	qName := os.Getenv("DOMAIN_EVENTS_QUEUE")
	queue, err := azqueue.NewQueueClientFromConnectionString(connStr, qName, nil)
	if err != nil {
		t.Fatalf("queue client: %v", err)
	}
	apiClient := newPrismApiClient(t)

	taskID := fmt.Sprintf("stale-%d", time.Now().UnixNano())
	userID := "integration-user"

	send := func(ev map[string]any) {
		b, _ := json.Marshal(ev)
		payload := map[string]any{"Data": map[string]any{"event": string(b)}}
		msg, _ := json.Marshal(payload)
		if _, err := queue.EnqueueMessage(context.Background(), string(msg), nil); err != nil {
			t.Fatalf("enqueue event: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
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
