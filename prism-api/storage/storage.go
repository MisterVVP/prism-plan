package storage

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
	"github.com/redis/go-redis/v9"

	"prism-api/domain"
)

type queueClient interface {
	EnqueueMessage(ctx context.Context, content string, o *azqueue.EnqueueMessageOptions) (azqueue.EnqueueMessagesResponse, error)
	GetProperties(ctx context.Context, o *azqueue.GetQueuePropertiesOptions) (azqueue.GetQueuePropertiesResponse, error)
}

type Storage struct {
	taskTable              *aztables.Client
	settingsTable          *aztables.Client
	commandQueue           queueClient
	taskPageSize           int32
	tasksSelectClause      string
	tasksSelectMetadataFmt aztables.MetadataFormat
	queueConcurrency       int
	cache                  redisGetter
}

// Option configures optional storage behaviors.
type Option func(*Storage)

// WithQueueConcurrency limits the number of concurrent queue requests issued when enqueuing commands.
func WithQueueConcurrency(n int) Option {
	return func(s *Storage) {
		if n > 0 {
			s.queueConcurrency = n
		}
	}
}

// WithCache configures Redis as a read-through cache for read models.
func WithCache(client redisGetter) Option {
	return func(s *Storage) {
		if client != nil {
			s.cache = client
		}
	}
}

const defaultQueueConcurrency = 8

// DefaultQueueConcurrency returns the default number of concurrent queue requests used when enqueuing commands.
func DefaultQueueConcurrency() int {
	return defaultQueueConcurrency
}

const maxTaskPageSize = int32(1000)

func New(connStr, tasksTable, settingsTable, commandQueue string, taskPageSize int, opts ...Option) (*Storage, error) {
	tablesClientOptions := aztables.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    3,
				TryTimeout:    time.Minute * 3,
				RetryDelay:    time.Second * 1,
				MaxRetryDelay: time.Second * 15,
				StatusCodes:   []int{408, 429, 500, 502, 503, 504},
			},
		},
	}
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, &tablesClientOptions)
	if err != nil {
		return nil, err
	}
	tt := svc.NewClient(tasksTable)
	st := svc.NewClient(settingsTable)
	queueClientOptions := azqueue.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    5,
				TryTimeout:    time.Minute * 5,
				RetryDelay:    time.Second * 1,
				MaxRetryDelay: time.Second * 60,
				StatusCodes:   []int{408, 429, 500, 502, 503, 504},
			},
		},
	}
	cq, err := azqueue.NewQueueClientFromConnectionString(connStr, commandQueue, &queueClientOptions)
	if err != nil {
		return nil, err
	}
	if taskPageSize <= 0 {
		return nil, fmt.Errorf("invalid task page size: %d", taskPageSize)
	}
	store := &Storage{
		taskTable:              tt,
		settingsTable:          st,
		commandQueue:           cq,
		taskPageSize:           int32(taskPageSize),
		tasksSelectClause:      "RowKey,Title,Notes,Category,Order,Done",
		tasksSelectMetadataFmt: aztables.MetadataFormatNone,
		queueConcurrency:       defaultQueueConcurrency,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}

	if store.queueConcurrency <= 0 {
		store.queueConcurrency = defaultQueueConcurrency
	}

	if store.taskPageSize <= 0 {
		store.taskPageSize = 1
	}
	if store.taskPageSize > maxTaskPageSize {
		store.taskPageSize = maxTaskPageSize
	}

	return store, nil
}

type taskEntity struct {
	aztables.Entity
	Title    string `json:"Title"`
	Notes    string `json:"Notes"`
	Category string `json:"Category"`
	Order    int    `json:"Order"`
	Done     bool   `json:"Done"`
}

type redisGetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
}

const (
	tasksCachePrefix    = "ts"
	settingsCachePrefix = "us"
)

type cachedTasks struct {
	Version       int           `json:"version"`
	CachedAt      time.Time     `json:"cachedAt"`
	LastUpdatedAt int64         `json:"lastUpdatedAt"`
	PageSize      int           `json:"pageSize"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
	Tasks         []domain.Task `json:"tasks"`
}

type cachedSettings struct {
	Version       int             `json:"version"`
	CachedAt      time.Time       `json:"cachedAt"`
	LastUpdatedAt int64           `json:"lastUpdatedAt"`
	Settings      domain.Settings `json:"settings"`
}

type continuationToken struct {
	PartitionKey string `json:"pk"`
	RowKey       string `json:"rk"`
}

type invalidContinuationTokenError struct {
	cause error
}

func (e *invalidContinuationTokenError) Error() string {
	if e == nil {
		return "invalid continuation token"
	}
	if e.cause == nil {
		return "invalid continuation token"
	}
	return "invalid continuation token: " + e.cause.Error()
}

func (e *invalidContinuationTokenError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *invalidContinuationTokenError) InvalidContinuationToken() {}

func decodeContinuationToken(token string) (*string, *string, error) {
	if token == "" {
		return nil, nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, nil, err
	}

	if len(data) >= 8 {
		pkLen := int(binary.BigEndian.Uint32(data[0:4]))
		rkLen := int(binary.BigEndian.Uint32(data[4:8]))
		expected := 8 + pkLen + rkLen
		if pkLen > 0 && rkLen > 0 && expected == len(data) {
			pk := string(data[8 : 8+pkLen])
			rk := string(data[8+pkLen : expected])
			if pk != "" && rk != "" {
				return &pk, &rk, nil
			}
		}
	}

	var ct continuationToken
	if err := json.Unmarshal(data, &ct); err != nil {
		return nil, nil, err
	}
	if ct.PartitionKey == "" || ct.RowKey == "" {
		return nil, nil, fmt.Errorf("missing continuation components")
	}
	pk := ct.PartitionKey
	rk := ct.RowKey
	return &pk, &rk, nil
}

func encodeContinuationToken(partitionKey, rowKey *string) (string, error) {
	if partitionKey == nil || rowKey == nil {
		return "", nil
	}
	if len(*partitionKey) == 0 || len(*rowKey) == 0 {
		return "", nil
	}

	pk := []byte(*partitionKey)
	rk := []byte(*rowKey)
	data := make([]byte, 8+len(pk)+len(rk))
	binary.BigEndian.PutUint32(data[0:4], uint32(len(pk)))
	binary.BigEndian.PutUint32(data[4:8], uint32(len(rk)))
	copy(data[8:], pk)
	copy(data[8+len(pk):], rk)

	return base64.RawURLEncoding.EncodeToString(data), nil
}

func resolveTaskPageSize(requested int, defaultSize int32) int32 {
	if defaultSize <= 0 {
		defaultSize = 1
	}
	if defaultSize > maxTaskPageSize {
		defaultSize = maxTaskPageSize
	}
	if requested <= 0 {
		return defaultSize
	}
	if requested > int(maxTaskPageSize) {
		return maxTaskPageSize
	}
	if requested < 1 {
		return defaultSize
	}
	return int32(requested)
}

func (s *Storage) FetchTasks(ctx context.Context, userID, token string, limit int) ([]domain.Task, string, error) {
	pageSize := resolveTaskPageSize(limit, s.taskPageSize)
	if token == "" && pageSize == s.taskPageSize {
		if cached, ok := s.loadTasksFromCache(ctx, userID); ok {
			if cached == nil {
				return []domain.Task{}, "", nil
			}
			tasks := cached.Tasks
			if tasks == nil {
				tasks = []domain.Task{}
			}
			return tasks, cached.NextPageToken, nil
		}
	}

	filter := "PartitionKey eq '" + userID + "'"
	nextPartitionKey, nextRowKey, err := decodeContinuationToken(token)
	if err != nil {
		return nil, "", &invalidContinuationTokenError{cause: err}
	}
	opts := &aztables.ListEntitiesOptions{Filter: &filter, Select: &s.tasksSelectClause, Top: &pageSize, Format: &s.tasksSelectMetadataFmt, NextPartitionKey: nextPartitionKey, NextRowKey: nextRowKey}
	pager := s.taskTable.NewListEntitiesPager(opts)
	if !pager.More() {
		return []domain.Task{}, "", nil
	}
	resp, err := pager.NextPage(ctx)
	if err != nil {
		return nil, "", err
	}
	tasks := make([]domain.Task, 0, len(resp.Entities))
	for _, e := range resp.Entities {
		var ent taskEntity
		if err := json.Unmarshal(e, &ent); err != nil {
			return nil, "", err
		}
		tasks = append(tasks, domain.Task{
			ID:       ent.RowKey,
			Title:    ent.Title,
			Notes:    ent.Notes,
			Category: ent.Category,
			Order:    ent.Order,
			Done:     ent.Done,
		})
	}
	nextToken, err := encodeContinuationToken(resp.NextPartitionKey, resp.NextRowKey)
	if err != nil {
		return nil, "", err
	}
	return tasks, nextToken, nil
}

func decodeSettingsEntity(data []byte) (domain.Settings, error) {
	var raw struct {
		TasksPerCategory int  `json:"TasksPerCategory"`
		ShowDoneTasks    bool `json:"ShowDoneTasks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.Settings{}, err
	}
	return domain.Settings{TasksPerCategory: raw.TasksPerCategory, ShowDoneTasks: raw.ShowDoneTasks}, nil
}

func (s *Storage) loadTasksFromCache(ctx context.Context, userID string) (*cachedTasks, bool) {
	if s.cache == nil {
		return nil, false
	}
	cmd := s.cache.Get(ctx, cacheKey(userID, tasksCachePrefix))
	raw, err := cmd.Result()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		log.Printf("storage: tasks cache lookup failed: %v", err)
		return nil, false
	}
	var payload cachedTasks
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		log.Printf("storage: tasks cache decode failed: %v", err)
		return nil, false
	}
	if payload.PageSize > 0 && int32(payload.PageSize) != s.taskPageSize {
		return nil, false
	}
	return &payload, true
}

func (s *Storage) loadSettingsFromCache(ctx context.Context, userID string) (*domain.Settings, bool) {
	if s.cache == nil {
		return nil, false
	}
	cmd := s.cache.Get(ctx, cacheKey(userID, settingsCachePrefix))
	raw, err := cmd.Result()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		log.Printf("storage: settings cache lookup failed: %v", err)
		return nil, false
	}
	var payload cachedSettings
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		log.Printf("storage: settings cache decode failed: %v", err)
		return nil, false
	}
	return &payload.Settings, true
}

func cacheKey(userID, prefix string) string {
	return userID + ":" + prefix
}

func (s *Storage) FetchSettings(ctx context.Context, userID string) (domain.Settings, error) {
	if settings, ok := s.loadSettingsFromCache(ctx, userID); ok {
		if settings == nil {
			return domain.Settings{}, nil
		}
		return *settings, nil
	}
	ent, err := s.settingsTable.GetEntity(ctx, userID, userID, &aztables.GetEntityOptions{Format: to.Ptr(aztables.MetadataFormatNone)})
	if err != nil {
		return domain.Settings{}, err
	}
	return decodeSettingsEntity(ent.Value)
}

func (s *Storage) Warmup(ctx context.Context) error {
	const warmupUserID = "__warmup__"

	if _, _, err := s.FetchTasks(ctx, warmupUserID, "", 0); err != nil {
		return err
	}

	if _, err := s.FetchSettings(ctx, warmupUserID); err != nil {
		var respErr *azcore.ResponseError
		if !errors.As(err, &respErr) || respErr.StatusCode != http.StatusNotFound {
			return err
		}
	}

	if _, err := s.commandQueue.GetProperties(ctx, nil); err != nil {
		return err
	}

	return nil
}

func (s *Storage) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	if len(cmds) == 0 {
		return nil
	}

	payloads := make([]string, len(cmds))
	for i, cmd := range cmds {
		env := domain.CommandEnvelope{UserID: userID, Command: cmd}
		data, err := json.Marshal(env)
		if err != nil {
			return err
		}
		payloads[i] = string(data)
	}

	workers := s.queueConcurrency
	if workers <= 1 {
		for _, payload := range payloads {
			if _, err := s.commandQueue.EnqueueMessage(ctx, payload, nil); err != nil {
				return err
			}
		}
		return nil
	}
	if workers > len(payloads) {
		workers = len(payloads)
	}

	parentCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan string, workers)
	var wg sync.WaitGroup
	var firstErr error
	var once sync.Once

	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case payload, ok := <-jobs:
				if !ok {
					return
				}
				if _, err := s.commandQueue.EnqueueMessage(ctx, payload, nil); err != nil {
					once.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
			}
		}
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker()
	}

loop:
	for _, payload := range payloads {
		select {
		case <-ctx.Done():
			break loop
		case jobs <- payload:
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	if err := parentCtx.Err(); err != nil {
		return err
	}
	return nil
}
