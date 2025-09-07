package scenarios

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"

	"prismtest/internal/httpclient"
)

type settings struct {
	TasksPerCategory int  `json:"tasksPerCategory"`
	ShowDoneTasks    bool `json:"showDoneTasks"`
}

func TestUserSettingsPersistence(t *testing.T) {
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

	userID := "integration-user"
	ts := time.Now().UnixNano()
        send(map[string]any{
                "Id":         "s1",
                "EntityId":   userID,
                "EntityType": "user-settings",
                "Type":       "user-settings-updated",
                "Timestamp":  ts,
                "UserId":     userID,
                "Data":       map[string]any{"showDoneTasks": true},
        })

        pollSettings(t, apiClient, "show done enabled", func(s settings) bool { return s.ShowDoneTasks })

        send(map[string]any{
                "Id":         "s2",
                "EntityId":   userID,
                "EntityType": "user-settings",
                "Type":       "user-settings-updated",
                "Timestamp":  ts + 1,
                "UserId":     userID,
                "Data":       map[string]any{"showDoneTasks": false},
        })

        pollSettings(t, apiClient, "show done disabled", func(s settings) bool { return !s.ShowDoneTasks })
}

func pollSettings(t *testing.T, client *httpclient.Client, desc string, cond func(settings) bool) settings {
	deadline := time.Now().Add(getPollTimeout(t))
	backoff := 200 * time.Millisecond
	var (
		st  settings
		err error
	)
	for {
		st = settings{}
		_, err = client.GetJSON("/api/settings", &st)
		if err == nil && cond(st) {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for settings for %s: last settings %+v: %v", desc, st, err)
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
}
