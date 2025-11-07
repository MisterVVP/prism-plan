package scenarios

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	idempotencyPartitionKey = "__idempotency__"
	completedStatus         = "Completed"
)

func TestIdempotencyCleanerRemovesCompletedKeys(t *testing.T) {
	t.Helper()
	client := newPrismApiClient(t)
	ctx := context.Background()

	table := newTaskEventsTableClient(t)

	title := fmt.Sprintf("cleanup-%d", time.Now().UnixNano())
	idempotencyKey := fmt.Sprintf("cleanup-%d", time.Now().UnixNano())
	payload := []command{{
		IdempotencyKey: idempotencyKey,
		EntityType:     "task",
		Type:           "create-task",
		Data: map[string]any{
			"title": title,
		},
	}}

	if resp, err := client.PostJSON("/api/commands", payload, nil); err != nil || resp.StatusCode >= http.StatusMultipleChoices {
		t.Fatalf("enqueue create-task command: status %d err %v", resp.StatusCode, err)
	}

	pollTasks(t, client, fmt.Sprintf("task with title %s to be created", title), func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				return true
			}
		}
		return false
	})

	waitForIdempotencyStatus(t, ctx, table, idempotencyKey, completedStatus)
	waitForIdempotencyDeletion(t, ctx, table, idempotencyKey)
}

func newTaskEventsTableClient(t *testing.T) *aztables.Client {
	t.Helper()
	connStr := os.Getenv("STORAGE_CONNECTION_STRING_LOCAL")
	if connStr == "" {
		t.Fatalf("STORAGE_CONNECTION_STRING_LOCAL must be set")
	}
	tableName := os.Getenv("TASK_EVENTS_TABLE")
	if tableName == "" {
		t.Fatalf("TASK_EVENTS_TABLE must be set")
	}
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("create table service client: %v", err)
	}
	return svc.NewClient(tableName)
}

func waitForIdempotencyStatus(t *testing.T, ctx context.Context, table *aztables.Client, rowKey, status string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	backoff := 200 * time.Millisecond
	for {
		got, notFound, err := loadIdempotencyStatus(ctx, table, rowKey)
		if err != nil {
			t.Fatalf("load idempotency status for %s: %v", rowKey, err)
		}
		if !notFound && got == status {
			return
		}
		if time.Now().After(deadline) {
			t.Logf("timeout waiting for idempotency key %s to reach status %s (last=%s, notFound=%t). Checking for key deletion next...", rowKey, status, got, notFound)
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
}

func waitForIdempotencyDeletion(t *testing.T, ctx context.Context, table *aztables.Client, rowKey string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	backoff := 200 * time.Millisecond
	for {
		_, notFound, err := loadIdempotencyStatus(ctx, table, rowKey)
		if err != nil {
			t.Fatalf("load idempotency status for %s: %v", rowKey, err)
		}
		if notFound {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for idempotency key %s to be deleted", rowKey)
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
}

func loadIdempotencyStatus(ctx context.Context, table *aztables.Client, rowKey string) (string, bool, error) {
	resp, err := table.GetEntity(ctx, idempotencyPartitionKey, rowKey, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
			return "", true, nil
		}
		return "", false, err
	}
	var payload struct {
		Status string `json:"Status"`
	}
	if err := json.Unmarshal(resp.Value, &payload); err != nil {
		return "", false, err
	}
	return payload.Status, false, nil
}
