package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDecodeSettingsEntity(t *testing.T) {
	testCases := map[string][]byte{
		"with_system_properties": []byte(`{"PartitionKey":"u1","RowKey":"u1","TasksPerCategory":5,"ShowDoneTasks":true}`),
		"metadata_removed":       []byte(`{"TasksPerCategory":5,"ShowDoneTasks":true}`),
	}
	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Helper()
			s, err := decodeSettingsEntity(data)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if s.TasksPerCategory != 5 || !s.ShowDoneTasks {
				t.Fatalf("unexpected settings: %+v", s)
			}
		})
	}
}

func TestTaskEntityDecodeWithoutMetadata(t *testing.T) {
	payload := []byte(`{"RowKey":"task1","Title":"Write tests","Notes":"cover metadata removal","Category":"dev","Order":3,"Done":false}`)
	var ent taskEntity
	if err := json.Unmarshal(payload, &ent); err != nil {
		t.Fatalf("unmarshal task entity: %v", err)
	}
	if ent.RowKey != "task1" {
		t.Fatalf("unexpected row key: %s", ent.RowKey)
	}
	if ent.Title != "Write tests" {
		t.Fatalf("unexpected title: %s", ent.Title)
	}
	if ent.Notes != "cover metadata removal" {
		t.Fatalf("unexpected notes: %s", ent.Notes)
	}
	if ent.Category != "dev" {
		t.Fatalf("unexpected category: %s", ent.Category)
	}
	if ent.Order != 3 {
		t.Fatalf("unexpected order: %d", ent.Order)
	}
	if ent.Done {
		t.Fatalf("unexpected done value: %t", ent.Done)
	}
}

func TestEncodeDecodeContinuationToken(t *testing.T) {
	pk := "p"
	rk := "r"
	token, err := encodeContinuationToken(&pk, &rk)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	dpk, drk, err := decodeContinuationToken(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dpk == nil || *dpk != pk {
		t.Fatalf("unexpected partition key: %v", dpk)
	}
	if drk == nil || *drk != rk {
		t.Fatalf("unexpected row key: %v", drk)
	}
}

func TestDecodeContinuationTokenInvalid(t *testing.T) {
	if _, _, err := decodeContinuationToken("not-base64"); err == nil {
		t.Fatal("expected error for malformed token")
	}
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"pk":""}`))
	if _, _, err := decodeContinuationToken(raw); err == nil {
		t.Fatal("expected error for missing components")
	}
}

func TestDecodeContinuationTokenLegacyJSON(t *testing.T) {
	pk := "legacy-pk"
	rk := "legacy-rk"
	legacy := continuationToken{PartitionKey: pk, RowKey: rk}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy token: %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(data)
	dpk, drk, err := decodeContinuationToken(token)
	if err != nil {
		t.Fatalf("decode legacy token: %v", err)
	}
	if dpk == nil || *dpk != pk {
		t.Fatalf("unexpected partition key: %v", dpk)
	}
	if drk == nil || *drk != rk {
		t.Fatalf("unexpected row key: %v", drk)
	}
}

func TestResolveTaskPageSize(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		requested   int
		defaultSize int32
		want        int32
	}{
		{name: "use_default_when_missing", requested: 0, defaultSize: 30, want: 30},
		{name: "respect_smaller_request", requested: 10, defaultSize: 30, want: 10},
		{name: "clamp_to_max", requested: 5000, defaultSize: 30, want: maxTaskPageSize},
		{name: "sanitize_negative", requested: -10, defaultSize: 25, want: 25},
		{name: "sanitize_default_zero", requested: 0, defaultSize: 0, want: 1},
		{name: "clamp_large_default", requested: 0, defaultSize: 2000, want: maxTaskPageSize},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveTaskPageSize(tc.requested, tc.defaultSize); got != tc.want {
				t.Fatalf("resolveTaskPageSize(%d, %d) = %d, want %d", tc.requested, tc.defaultSize, got, tc.want)
			}
		})
	}
}

type stubRedisGetter struct {
	value   string
	err     error
	lastKey string
}

func (s *stubRedisGetter) Get(ctx context.Context, key string) *redis.StringCmd {
	s.lastKey = key
	return redis.NewStringResult(s.value, s.err)
}

func TestFetchTasksUsesCache(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	cacheValue := `{"version":2,"cachedAt":"` + now + `","lastUpdatedAt":1,"pageSize":3,"pages":{"":` +
		`{"tasks":[{"id":"t1","title":"Task","category":"cat","order":1}],"nextPageToken":"abc"},` +
		`"abc":{"tasks":[{"id":"t2","title":"Task 2","category":"cat","order":2}],"nextPageToken":""}}}`
	cache := &stubRedisGetter{value: cacheValue}
	store := &Storage{taskPageSize: 3, cache: cache}

	tasks, token, err := store.FetchTasks(context.Background(), "user", "", 0)
	if err != nil {
		t.Fatalf("FetchTasks first page: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("unexpected first page tasks: %+v", tasks)
	}
	if token != "abc" {
		t.Fatalf("unexpected token for first page: %s", token)
	}

	tasks, token, err = store.FetchTasks(context.Background(), "user", token, 0)
	if err != nil {
		t.Fatalf("FetchTasks second page: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t2" {
		t.Fatalf("unexpected second page tasks: %+v", tasks)
	}
	if token != "" {
		t.Fatalf("unexpected token for second page: %s", token)
	}
	if cache.lastKey != cacheKey("user", tasksCachePrefix) {
		t.Fatalf("unexpected cache key: %s", cache.lastKey)
	}
}

func TestFetchSettingsUsesCache(t *testing.T) {
	cacheValue := `{"version":1,"cachedAt":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","lastUpdatedAt":1,"settings":{"tasksPerCategory":4,"showDoneTasks":true}}`
	cache := &stubRedisGetter{value: cacheValue}
	store := &Storage{cache: cache}
	settings, err := store.FetchSettings(context.Background(), "user")
	if err != nil {
		t.Fatalf("FetchSettings: %v", err)
	}
	if settings.TasksPerCategory != 4 || !settings.ShowDoneTasks {
		t.Fatalf("unexpected settings: %+v", settings)
	}
	if cache.lastKey != cacheKey("user", settingsCachePrefix) {
		t.Fatalf("unexpected cache key: %s", cache.lastKey)
	}
}

func TestLoadTasksFromCacheInvalidJSON(t *testing.T) {
	cache := &stubRedisGetter{value: "not-json"}
	store := &Storage{taskPageSize: 5, cache: cache}
	if cached, ok := store.loadTasksFromCache(context.Background(), "user"); ok || cached != nil {
		t.Fatalf("expected cache miss on invalid json")
	}
}

func TestLoadSettingsFromCacheInvalidJSON(t *testing.T) {
	cache := &stubRedisGetter{value: "not-json"}
	store := &Storage{cache: cache}
	if cached, ok := store.loadSettingsFromCache(context.Background(), "user"); ok || cached != nil {
		t.Fatalf("expected cache miss on invalid json")
	}
}
