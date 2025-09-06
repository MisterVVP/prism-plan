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

func TestReopenCompletedTasks(t *testing.T) {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING_LOCAL")
	qName := os.Getenv("DOMAIN_EVENTS_QUEUE")
	queue, err := azqueue.NewQueueClientFromConnectionString(connStr, qName, nil)
	if err != nil {
		t.Fatalf("queue client: %v", err)
	}
	apiClient := newPrismApiClient(t)

	send := func(ev map[string]any) {
		b, _ := json.Marshal(ev)
		if _, err := queue.EnqueueMessage(context.Background(), string(b), nil); err != nil {
			t.Fatalf("enqueue event: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	base := time.Now().UnixNano()
	userID := "integration-user"
	categories := []string{"cat1", "cat2", "cat3", "cat4"}
	ids := make([]string, len(categories))

	// create tasks
	for i, cat := range categories {
		id := fmt.Sprintf("reopen-%d-%d", i, base)
		ids[i] = id
		send(map[string]any{
			"Id":         fmt.Sprintf("c%d", i),
			"EntityId":   id,
			"EntityType": "task",
			"Type":       "task-created",
			"Timestamp":  base + int64(i),
			"UserId":     userID,
			"Data":       map[string]any{"title": id, "category": cat, "order": i},
		})
	}

	ts := base + 1000
	// move tasks to done
	for i, id := range ids {
		send(map[string]any{
			"Id":         fmt.Sprintf("d%d", i),
			"EntityId":   id,
			"EntityType": "task",
			"Type":       "task-updated",
			"Timestamp":  ts + int64(i),
			"UserId":     userID,
			"Data":       map[string]any{"category": "done", "order": i},
		})
		send(map[string]any{
			"Id":         fmt.Sprintf("dc%d", i),
			"EntityId":   id,
			"EntityType": "task",
			"Type":       "task-updated",
			"Timestamp":  ts + int64(i) + 1,
			"UserId":     userID,
			"Data":       map[string]any{"done": true},
		})
	}

	ts2 := ts + 1000
	// reopen tasks
	for i, id := range ids {
		send(map[string]any{
			"Id":         fmt.Sprintf("r%d", i),
			"EntityId":   id,
			"EntityType": "task",
			"Type":       "task-updated",
			"Timestamp":  ts2 + int64(i),
			"UserId":     userID,
			"Data":       map[string]any{"category": categories[i], "order": i, "done": false},
		})
	}

	// stale completion events arrive late
	for i, id := range ids {
		send(map[string]any{
			"Id":         fmt.Sprintf("s%d", i),
			"EntityId":   id,
			"EntityType": "task",
			"Type":       "task-updated",
			"Timestamp":  ts + int64(i),
			"UserId":     userID,
			"Data":       map[string]any{"done": true},
		})
	}

	pollTasks(t, apiClient, "tasks reopened", func(tsks []task) bool {
		found := 0
		for i, id := range ids {
			for _, tk := range tsks {
				if tk.ID == id && tk.Category == categories[i] && !tk.Done {
					found++
				}
			}
		}
		return found == len(ids)
	})
}
