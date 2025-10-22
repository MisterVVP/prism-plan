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
	"github.com/redis/go-redis/v9"

	testutil "prismtestutil"
)

type cachedTaskEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type cachedTasksPayload struct {
	PageSize int               `json:"pageSize"`
	Tasks    []cachedTaskEntry `json:"tasks"`
}

func TestTasksServedFromCache(t *testing.T) {
	ctx := context.Background()
	userID := fmt.Sprintf("cache-user-%d", time.Now().UnixNano())
	bearer, err := testutil.TestToken(userID)
	if err != nil {
		t.Fatalf("generate bearer: %v", err)
	}
	t.Setenv("TEST_BEARER", bearer)

	client := newPrismApiClient(t)
	redisClient := newRedisClient(t)
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	cacheKey := cacheKeyForUser(userID)
	if err := redisClient.Del(ctx, cacheKey).Err(); err != nil && err != redis.Nil {
		t.Fatalf("clear cache: %v", err)
	}

	titles := []string{"cache-task-a", "cache-task-b"}
	for i, title := range titles {
		payload := []command{{
			IdempotencyKey: fmt.Sprintf("ik-cache-%d-%d", i, time.Now().UnixNano()),
			EntityType:     "task",
			Type:           "create-task",
			Data: map[string]any{
				"title": title,
			},
		}}
		resp, err := client.PostJSON("/api/commands", payload, nil)
		if err != nil || resp.StatusCode >= http.StatusBadRequest {
			status := 0
			if resp != nil {
				status = resp.StatusCode
			}
			t.Fatalf("create task %d: status %d err %v", i, status, err)
		}
	}

	tasks := pollTasks(t, client, "tasks projected", func(ts []task) bool {
		if len(ts) < len(titles) {
			return false
		}
		seen := make(map[string]bool, len(ts))
		for _, tk := range ts {
			seen[tk.Title] = true
		}
		for _, title := range titles {
			if !seen[title] {
				return false
			}
		}
		return true
	})

	cached := waitForCachedTasks(t, ctx, redisClient, cacheKey, len(titles))
	if cached.PageSize == 0 {
		t.Fatalf("expected cached page size to be non-zero")
	}

	deleted := deleteAllUserTasksFromTable(t, ctx, userID)
	if deleted == 0 {
		t.Fatalf("expected to delete at least one task entity for user %s", userID)
	}
	ensureNoTasksRemainInTable(t, ctx, userID)

	var page taskPage
	resp, err := client.GetJSON("/api/tasks", &page)
	if err != nil {
		t.Fatalf("fetch tasks after table deletion: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetch tasks after table deletion: status %d", resp.StatusCode)
	}
	if len(page.Tasks) != len(tasks) {
		t.Fatalf("expected %d tasks from cache, got %d", len(tasks), len(page.Tasks))
	}

	expected := make(map[string]string, len(cached.Tasks))
	for _, entry := range cached.Tasks {
		expected[entry.ID] = entry.Title
	}
	for _, tk := range page.Tasks {
		title, ok := expected[tk.ID]
		if !ok {
			t.Fatalf("task %s missing from cached expectations", tk.ID)
		}
		if title != tk.Title {
			t.Fatalf("task %s title mismatch: cache %s api %s", tk.ID, title, tk.Title)
		}
	}
}

func newRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	conn := os.Getenv("REDIS_CONNECTION_STRING")
	if conn == "" {
		t.Fatalf("REDIS_CONNECTION_STRING must be set for cache test")
	}
	opts, err := redis.ParseURL(conn)
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}
	return redis.NewClient(opts)
}

func cacheKeyForUser(userID string) string {
	return userID + ":ts"
}

func waitForCachedTasks(t *testing.T, ctx context.Context, client *redis.Client, key string, want int) cachedTasksPayload {
	t.Helper()
	deadline := time.Now().Add(getPollTimeout(t))
	backoff := 200 * time.Millisecond
	for {
		raw, err := client.Get(ctx, key).Result()
		if err == nil {
			var payload cachedTasksPayload
			if err := json.Unmarshal([]byte(raw), &payload); err == nil {
				if len(payload.Tasks) >= want {
					return payload
				}
			}
		} else if err != redis.Nil {
			t.Fatalf("redis get %s: %v", key, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for cache key %s to contain %d tasks", key, want)
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
}

func deleteAllUserTasksFromTable(t *testing.T, ctx context.Context, userID string) int {
	t.Helper()
	connStr := os.Getenv("STORAGE_CONNECTION_STRING_LOCAL")
	if connStr == "" {
		t.Fatalf("STORAGE_CONNECTION_STRING_LOCAL must be set for cache test")
	}
	tableName := os.Getenv("TASKS_TABLE")
	if tableName == "" {
		t.Fatalf("TASKS_TABLE must be set for cache test")
	}
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("service client: %v", err)
	}
	table := svc.NewClient(tableName)
	filter := fmt.Sprintf("PartitionKey eq '%s'", userID)
	format := aztables.MetadataFormatNone
	pager := table.NewListEntitiesPager(&aztables.ListEntitiesOptions{Filter: &filter, Format: &format})
	deleted := 0
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			t.Fatalf("list tasks: %v", err)
		}
		for _, entity := range page.Entities {
			var row struct {
				RowKey string `json:"RowKey"`
			}
			if err := json.Unmarshal(entity, &row); err != nil {
				t.Fatalf("decode entity: %v", err)
			}
			if row.RowKey == "" {
				continue
			}
			match := azcore.ETagAny
			if _, err := table.DeleteEntity(ctx, userID, row.RowKey, &aztables.DeleteEntityOptions{IfMatch: &match}); err != nil {
				t.Fatalf("delete entity %s: %v", row.RowKey, err)
			}
			deleted++
		}
	}
	return deleted
}

func ensureNoTasksRemainInTable(t *testing.T, ctx context.Context, userID string) {
	t.Helper()
	connStr := os.Getenv("STORAGE_CONNECTION_STRING_LOCAL")
	tableName := os.Getenv("TASKS_TABLE")
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		t.Fatalf("service client: %v", err)
	}
	table := svc.NewClient(tableName)
	filter := fmt.Sprintf("PartitionKey eq '%s'", userID)
	format := aztables.MetadataFormatNone
	pager := table.NewListEntitiesPager(&aztables.ListEntitiesOptions{Filter: &filter, Format: &format})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			t.Fatalf("list tasks: %v", err)
		}
		if len(page.Entities) > 0 {
			t.Fatalf("expected no task entities remaining for user %s", userID)
		}
	}
}
