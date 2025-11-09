package storage

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/bytedance/sonic"
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
	if err := sonic.Unmarshal(payload, &ent); err != nil {
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
	data, err := sonic.Marshal(legacy)
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

type stubbedStorage struct {
	Storage
	loader func(ctx context.Context, userID string) (*cachedTasks, bool)
}

func (s *stubbedStorage) loadTasksFromCache(ctx context.Context, userID string) (*cachedTasks, bool) {
	if s.loader != nil {
		return s.loader(ctx, userID)
	}
	return s.Storage.loadTasksFromCache(ctx, userID)
}

func TestFetchTasksUsesCache(t *testing.T) {
	cacheValue := `{"version":1,"cachedAt":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","lastUpdatedAt":1,"pageSize":3,"cachedPages":1,"nextPageToken":"abc","tasks":[{"id":"t1","title":"Task","category":"cat","order":1}]}`
	cache := &stubRedisGetter{value: cacheValue}
	store := &Storage{
		taskPageSize: 3,
		cache:        cache,
	}
	tasks, token, err := store.FetchTasks(context.Background(), "user", "", 0)
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "t1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if token != "abc" {
		t.Fatalf("unexpected token: %s", token)
	}
	if cache.lastKey != cacheKey("user", tasksCachePrefix) {
		t.Fatalf("unexpected cache key: %s", cache.lastKey)
	}
}

func TestFetchTasksUsesCacheMultiplePages(t *testing.T) {
	pk := "user"
	rkFirst := "t3"
	rkSecond := "t6"
	tokenSecondPage, err := encodeContinuationToken(&pk, &rkFirst)
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	tokenAfterCache, err := encodeContinuationToken(&pk, &rkSecond)
	if err != nil {
		t.Fatalf("encode final token: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	cacheValue := `{"version":1,"cachedAt":"` + now + `","lastUpdatedAt":1,"pageSize":3,"cachedPages":2,"pageTokens":["` + tokenSecondPage + `"],"nextPageToken":"` + tokenAfterCache + `","tasks":[` +
		`{"id":"t1","title":"T1","category":"c","order":1},` +
		`{"id":"t2","title":"T2","category":"c","order":2},` +
		`{"id":"t3","title":"T3","category":"c","order":3},` +
		`{"id":"t4","title":"T4","category":"c","order":4},` +
		`{"id":"t5","title":"T5","category":"c","order":5},` +
		`{"id":"t6","title":"T6","category":"c","order":6}` +
		`]}`
	cache := &stubRedisGetter{value: cacheValue}
	store := &Storage{taskPageSize: 3, cache: cache}

	tasks, token, err := store.FetchTasks(context.Background(), "user", "", 0)
	if err != nil {
		t.Fatalf("FetchTasks first page: %v", err)
	}
	if len(tasks) != 3 || tasks[0].ID != "t1" || tasks[2].ID != "t3" {
		t.Fatalf("unexpected first page: %+v", tasks)
	}
	if token != tokenSecondPage {
		t.Fatalf("unexpected next token after first page: %s", token)
	}

	tasks, token, err = store.FetchTasks(context.Background(), "user", tokenSecondPage, 0)
	if err != nil {
		t.Fatalf("FetchTasks second page: %v", err)
	}
	if len(tasks) != 3 || tasks[0].ID != "t4" || tasks[2].ID != "t6" {
		t.Fatalf("unexpected second page: %+v", tasks)
	}
	if token != tokenAfterCache {
		t.Fatalf("unexpected final token: %s", token)
	}
}

func TestFetchTasksCacheEmptyFallsBackToTable(t *testing.T) {
	cacheValue := `{"version":1,"cachedAt":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","lastUpdatedAt":1,"pageSize":3,"cachedPages":1,"nextPageToken":"abc","tasks":[]}`
	cache := &stubRedisGetter{value: cacheValue}
	store := &Storage{taskPageSize: 3, cache: cache}

	tasks, token, ok := store.fetchTasksFromCache(context.Background(), "user", "", store.taskPageSize)
	if ok {
		t.Fatalf("expected cache fallback, got tasks=%v token=%q", tasks, token)
	}
	if tasks != nil {
		t.Fatalf("expected no tasks when falling back, got %#v", tasks)
	}
	if token != "" {
		t.Fatalf("expected empty token when falling back, got %q", token)
	}
}

func TestFetchTasksCacheNilPayloadFallsBackToTable(t *testing.T) {
	store := &stubbedStorage{
		Storage: Storage{taskPageSize: 3},
		loader: func(ctx context.Context, userID string) (*cachedTasks, bool) {
			return nil, true
		},
	}

	tasks, token, ok := store.fetchTasksFromCache(context.Background(), "user", "", store.taskPageSize)
	if ok {
		t.Fatalf("expected cache fallback when payload nil, got ok with tasks=%v token=%q", tasks, token)
	}
	if tasks != nil {
		t.Fatalf("expected nil tasks on fallback, got %#v", tasks)
	}
	if token != "" {
		t.Fatalf("expected empty token on fallback, got %q", token)
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
