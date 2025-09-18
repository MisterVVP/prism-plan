package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue/queueerror"
)

func TestCommandQueueFallbackWhenDomainEventsQueueUnavailable(t *testing.T) {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING_LOCAL")
	commandQueueName := os.Getenv("COMMAND_QUEUE")
	domainEventsQueueName := os.Getenv("DOMAIN_EVENTS_QUEUE")
	if connStr == "" || commandQueueName == "" || domainEventsQueueName == "" {
		t.Skip("azure storage configuration missing")
	}

	ctx := context.Background()

	eventsQueue, err := azqueue.NewQueueClientFromConnectionString(connStr, domainEventsQueueName, nil)
	if err != nil {
		t.Fatalf("domain events queue client: %v", err)
	}
	t.Cleanup(func() {
		// Restore queue availability for subsequent scenarios.
		if _, err := eventsQueue.Create(context.Background(), nil); err != nil && !queueerror.HasCode(err, queueerror.QueueAlreadyExists) {
			t.Fatalf("restore domain events queue: %v", err)
		}
	})

	if _, err := eventsQueue.Delete(ctx, nil); err != nil && !queueerror.HasCode(err, queueerror.QueueNotFound, queueerror.QueueBeingDeleted) {
		t.Fatalf("delete domain events queue: %v", err)
	}

	commandQueue, err := azqueue.NewQueueClientFromConnectionString(connStr, commandQueueName, nil)
	if err != nil {
		t.Fatalf("command queue client: %v", err)
	}

	title := fmt.Sprintf("fallback-queue-title-%d", time.Now().UnixNano())
	idempotencyKey := fmt.Sprintf("ik-fallback-%d", time.Now().UnixNano())

	envelope := map[string]any{
		"userId": "integration-user",
		"command": map[string]any{
			"id":             idempotencyKey,
			"idempotencyKey": idempotencyKey,
			"entityType":     "task",
			"type":           "create-task",
			"timestamp":      time.Now().UnixMilli(),
			"data": map[string]any{
				"title": title,
			},
		},
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}

	if _, err := commandQueue.EnqueueMessage(ctx, string(payload), nil); err != nil {
		t.Fatalf("enqueue command: %v", err)
	}

	client := newPrismApiClient(t)

	pollTasks(t, client, fmt.Sprintf("task %s to appear via fallback", title), func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				return true
			}
		}
		return false
	})

	if _, err := eventsQueue.GetProperties(ctx, nil); err == nil || !queueerror.HasCode(err, queueerror.QueueNotFound, queueerror.QueueBeingDeleted) {
		t.Fatalf("domain events queue unexpectedly reachable: %v", err)
	}
}
