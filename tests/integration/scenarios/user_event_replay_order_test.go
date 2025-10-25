package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"

	"prismtest/internal/httpclient"
	testutil "prismtestutil"
)

func TestUserEventReplayPreservesOriginalOrder(t *testing.T) {
	ctx := context.Background()
	connStr := os.Getenv("STORAGE_CONNECTION_STRING_LOCAL")
	if connStr == "" {
		t.Skip("STORAGE_CONNECTION_STRING_LOCAL not set")
	}
	userEventsTable := os.Getenv("USER_EVENTS_TABLE")
	if userEventsTable == "" {
		t.Fatalf("USER_EVENTS_TABLE must be set")
	}
	domainEventsQueue := os.Getenv("DOMAIN_EVENTS_QUEUE")
	if domainEventsQueue == "" {
		t.Fatalf("DOMAIN_EVENTS_QUEUE must be set")
	}

	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("service client: %v", err)
	}
	table := svc.NewClient(userEventsTable)

	queue, err := azqueue.NewQueueClientFromConnectionString(connStr, domainEventsQueue, nil)
	if err != nil {
		t.Fatalf("queue client: %v", err)
	}
	if _, err := queue.ClearMessages(ctx, nil); err != nil {
		t.Fatalf("clear queue: %v", err)
	}

	ts := time.Now().UnixMilli()
	userID := fmt.Sprintf("replay-user-%d", ts)
	idemKey := fmt.Sprintf("ik-login-%d", ts)
	userRow := "0002-user"
	settingsRow := "0001-settings"

	userData, err := json.Marshal(map[string]any{"name": "Replay User", "email": "replay@example.com"})
	if err != nil {
		t.Fatalf("marshal user data: %v", err)
	}
	settingsData, err := json.Marshal(map[string]any{"tasksPerCategory": 3, "showDoneTasks": false})
	if err != nil {
		t.Fatalf("marshal settings data: %v", err)
	}

	insertedAtUser := time.UnixMilli(ts).UTC().Format(time.RFC3339Nano)
	insertedAtSettings := time.UnixMilli(ts + 1).UTC().Format(time.RFC3339Nano)

	userEntity := map[string]any{
		"PartitionKey":          userID,
		"RowKey":                userRow,
		"Type":                  "user-created",
		"EventTimestamp":        ts,
		"UserId":                userID,
		"IdempotencyKey":        idemKey,
		"EntityType":            "user",
		"Dispatched":            false,
		"Data":                  string(userData),
		"InsertedAt":            insertedAtUser,
		"InsertedAt@odata.type": "Edm.DateTimeOffset",
	}
	settingsEntity := map[string]any{
		"PartitionKey":          userID,
		"RowKey":                settingsRow,
		"Type":                  "user-settings-created",
		"EventTimestamp":        ts,
		"UserId":                userID,
		"IdempotencyKey":        idemKey,
		"EntityType":            "user-settings",
		"Dispatched":            false,
		"Data":                  string(settingsData),
		"InsertedAt":            insertedAtSettings,
		"InsertedAt@odata.type": "Edm.DateTimeOffset",
	}

	userPayload, err := json.Marshal(userEntity)
	if err != nil {
		t.Fatalf("marshal user entity: %v", err)
	}
	if _, err := table.AddEntity(ctx, userPayload, nil); err != nil {
		t.Fatalf("insert user event: %v", err)
	}
	settingsPayload, err := json.Marshal(settingsEntity)
	if err != nil {
		t.Fatalf("marshal settings entity: %v", err)
	}
	if _, err := table.AddEntity(ctx, settingsPayload, nil); err != nil {
		t.Fatalf("insert settings event: %v", err)
	}

	t.Cleanup(func() {
		match := azcore.ETagAny
		_, _ = table.DeleteEntity(ctx, userID, userRow, &aztables.DeleteEntityOptions{IfMatch: &match})
		_, _ = table.DeleteEntity(ctx, userID, settingsRow, &aztables.DeleteEntityOptions{IfMatch: &match})
		_, _ = table.DeleteEntity(ctx, "__idempotency__", idemKey, &aztables.DeleteEntityOptions{IfMatch: &match})
	})

	bearer, err := testutil.TestToken(userID)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	base := os.Getenv("PRISM_API_LB_BASE")
	if base == "" {
		t.Fatalf("PRISM_API_LB_BASE must be set")
	}
	health := os.Getenv("API_HEALTH_ENDPOINT")
	if health == "" {
		t.Fatalf("API_HEALTH_ENDPOINT must be set")
	}
	if _, err := http.Get(base + health); err != nil {
		t.Fatalf("API not reachable: %v", err)
	}
	client := httpclient.New(base, bearer)

	payload := []command{{
		IdempotencyKey: idemKey,
		EntityType:     "user",
		Type:           "login-user",
		Data: map[string]any{
			"name":  "Replay User",
			"email": "replay@example.com",
		},
	}}
	if resp, err := client.PostJSON("/api/commands", payload, nil); err != nil || resp.StatusCode >= 300 {
		t.Fatalf("enqueue login command: status %d err %v", resp.StatusCode, err)
	}

	want := []string{"user-created", "user-settings-created"}
	got := collectEventTypes(t, ctx, queue, idemKey, len(want))
	if len(got) != len(want) {
		t.Fatalf("expected %d events, got %d (%v)", len(want), len(got), got)
	}
	for i, typ := range want {
		if got[i] != typ {
			t.Fatalf("event %d type mismatch: want %s got %s", i, typ, got[i])
		}
	}
}

func collectEventTypes(t *testing.T, ctx context.Context, queue *azqueue.QueueClient, idemKey string, want int) []string {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	backoff := 200 * time.Millisecond
	results := make([]string, 0, want)
	for len(results) < want {
		num := int32(want - len(results))
		resp, err := queue.DequeueMessages(ctx, &azqueue.DequeueMessagesOptions{NumberOfMessages: &num})
		if err != nil {
			t.Fatalf("dequeue messages: %v", err)
		}
		for _, msg := range resp.Messages {
			if msg.MessageText == nil {
				continue
			}
			var payload struct {
				Type           string `json:"Type"`
				IdempotencyKey string `json:"IdempotencyKey"`
			}
			if err := json.Unmarshal([]byte(*msg.MessageText), &payload); err != nil {
				t.Fatalf("decode message: %v", err)
			}
			if msg.MessageID != nil && msg.PopReceipt != nil {
				_, _ = queue.DeleteMessage(ctx, *msg.MessageID, *msg.PopReceipt, nil)
			}
			if payload.IdempotencyKey == idemKey {
				results = append(results, payload.Type)
			}
		}
		if len(results) >= want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %d events, got %d", want, len(results))
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
	return results
}
