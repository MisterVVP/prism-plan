package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"

	"prism-api/domain"
)

// Storage provides access to underlying persistence mechanisms.
type Storage struct {
	taskTable     *aztables.Client
	settingsTable *aztables.Client
	commandQueue  *azqueue.QueueClient
	taskPageSize  int32
}

// New creates a Storage instance from the given connection string.
func New(connStr, tasksTable, settingsTable, commandQueue string, taskPageSize int) (*Storage, error) {
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
	return &Storage{taskTable: tt, settingsTable: st, commandQueue: cq, taskPageSize: int32(taskPageSize)}, nil
}

type taskEntity struct {
	aztables.Entity
	Title    string `json:"Title"`
	Notes    string `json:"Notes"`
	Category string `json:"Category"`
	Order    int    `json:"Order"`
	Done     bool   `json:"Done"`
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
	data, err := json.Marshal(continuationToken{PartitionKey: *partitionKey, RowKey: *rowKey})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// FetchTasks retrieves a single page of tasks for the provided user and returns a continuation token when more results are available.
func (s *Storage) FetchTasks(ctx context.Context, userID, token string) ([]domain.Task, string, error) {
	filter := "PartitionKey eq '" + userID + "'"
	nextPartitionKey, nextRowKey, err := decodeContinuationToken(token)
	if err != nil {
		return nil, "", &invalidContinuationTokenError{cause: err}
	}
	top := s.taskPageSize
	opts := &aztables.ListEntitiesOptions{Filter: &filter, Top: &top, NextPartitionKey: nextPartitionKey, NextRowKey: nextRowKey}
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

func (s *Storage) FetchSettings(ctx context.Context, userID string) (domain.Settings, error) {
	ent, err := s.settingsTable.GetEntity(ctx, userID, userID, nil)
	if err != nil {
		return domain.Settings{}, err
	}
	return decodeSettingsEntity(ent.Value)
}

// EnqueueCommands sends the given commands to the command queue.
func (s *Storage) EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error {
	for _, cmd := range cmds {
		env := domain.CommandEnvelope{UserID: userID, Command: cmd}
		data, err := json.Marshal(env)
		if err != nil {
			return err
		}
		if _, err := s.commandQueue.EnqueueMessage(ctx, string(data), nil); err != nil {
			return err
		}
	}
	return nil
}
