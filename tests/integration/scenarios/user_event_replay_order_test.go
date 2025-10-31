package scenarios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"

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
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("service client: %v", err)
	}
	table := svc.NewClient(userEventsTable)

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
		"PartitionKey":   userID,
		"RowKey":         userRow,
		"Type":           "user-created",
		"EventTimestamp": ts,
		"UserId":         userID,
		"IdempotencyKey": idemKey,
		"EntityType":     "user",
		"Dispatched":     false,
		"Data":           string(userData),
		"InsertedAt":     insertedAtUser,
	}
	settingsEntity := map[string]any{
		"PartitionKey":   userID,
		"RowKey":         settingsRow,
		"Type":           "user-settings-created",
		"EventTimestamp": ts + 1,
		"UserId":         userID,
		"IdempotencyKey": idemKey,
		"EntityType":     "user-settings",
		"Dispatched":     false,
		"Data":           string(settingsData),
		"InsertedAt":     insertedAtSettings,
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
	events := collectReplayEvents(t, ctx, table, userID, []string{userRow, settingsRow}, idemKey)
	if len(events) != len(want) {
		t.Fatalf("expected %d events, got %d (%v)", len(want), len(events), events)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].InsertedAt.Equal(events[j].InsertedAt) {
			return !events[i].DispatchedAt.After(events[j].DispatchedAt)
		}
		return events[i].InsertedAt.Before(events[j].InsertedAt)
	})
	for i, typ := range want {
		if events[i].Type != typ {
			t.Fatalf("event %d type mismatch: want %s got %s", i, typ, events[i].Type)
		}
	}
	if len(events) >= 2 && events[0].DispatchedAt.After(events[1].DispatchedAt) {
		t.Fatalf("expected %s to be dispatched before %s (%s > %s)", events[0].Type, events[1].Type, events[0].DispatchedAt, events[1].DispatchedAt)
	}
}

type replayedEvent struct {
	Type         string
	InsertedAt   time.Time
	DispatchedAt time.Time
}

func collectReplayEvents(t *testing.T, ctx context.Context, table *aztables.Client, userID string, rows []string, idemKey string) []replayedEvent {
	t.Helper()
	events := make([]replayedEvent, 0, len(rows))
	for _, rk := range rows {
		events = append(events, waitForDispatchedEvent(t, ctx, table, userID, rk, idemKey))
	}
	return events
}

func waitForDispatchedEvent(t *testing.T, ctx context.Context, table *aztables.Client, pk, rk, idemKey string) replayedEvent {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	backoff := 200 * time.Millisecond
	for {
		resp, err := table.GetEntity(ctx, pk, rk, nil)
		if err != nil {
			t.Fatalf("get entity %s/%s: %v", pk, rk, err)
		}
		var raw struct {
			Type           string               `json:"Type"`
			IdempotencyKey string               `json:"IdempotencyKey"`
			Dispatched     bool                 `json:"Dispatched"`
			InsertedAt     string               `json:"InsertedAt"`
			Timestamp      aztables.EDMDateTime `json:"Timestamp"`
		}
		if err := json.Unmarshal(resp.Value, &raw); err != nil {
			t.Fatalf("decode entity %s/%s: %v", pk, rk, err)
		}
		if raw.IdempotencyKey != idemKey {
			t.Fatalf("unexpected idempotency key for %s/%s: want %s got %s", pk, rk, idemKey, raw.IdempotencyKey)
		}
		insertedAt, err := time.Parse(time.RFC3339Nano, raw.InsertedAt)
		if err != nil {
			t.Fatalf("parse InsertedAt for %s/%s: %v", pk, rk, err)
		}
		dispatchedAt := time.Time(raw.Timestamp)
		if !raw.Dispatched || dispatchedAt.IsZero() {
			if time.Now().After(deadline) {
				t.Fatalf("timeout waiting for event %s/%s to be dispatched", pk, rk)
			}
			time.Sleep(backoff)
			if backoff < time.Second {
				backoff *= 2
			}
			continue
		}
		return replayedEvent{Type: raw.Type, InsertedAt: insertedAt, DispatchedAt: dispatchedAt}
	}
}
