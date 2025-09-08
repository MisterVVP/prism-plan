package storage

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"

	"read-model-updater/domain"
)

// Storage wraps Azure clients used by the service.
type Storage struct {
	queue         *azqueue.QueueClient
	taskTable     *aztables.Client
	userTable     *aztables.Client
	settingsTable *aztables.Client
}

func parseTimestamp(raw json.RawMessage) int64 {
	var i int64
	if err := json.Unmarshal(raw, &i); err == nil {
		return i
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if t, err2 := time.Parse(time.RFC3339Nano, s); err2 == nil {
			return t.UnixNano()
		}
		if i, err2 := strconv.ParseInt(s, 10, 64); err2 == nil {
			return i
		}
	}
	return 0
}

// New creates a Storage from connection parameters.
func New(connStr, eventsQueue, tasksTable, usersTable, settingsTable string) (*Storage, error) {
	queue, err := azqueue.NewQueueClientFromConnectionString(connStr, eventsQueue, nil)
	if err != nil {
		return nil, err
	}
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		return nil, err
	}
	taskClient := svc.NewClient(tasksTable)
	userClient := svc.NewClient(usersTable)
	settingsClient := svc.NewClient(settingsTable)
	return &Storage{queue: queue, taskTable: taskClient, userTable: userClient, settingsTable: settingsClient}, nil
}

// Dequeue retrieves a single message from the events queue.
func (s *Storage) Dequeue(ctx context.Context) (*azqueue.DequeuedMessage, error) {
	resp, err := s.queue.DequeueMessage(ctx, nil)
	if err != nil {
		return nil, err
	}
	if len(resp.Messages) == 0 {
		return nil, nil
	}
	return resp.Messages[0], nil
}

// Delete removes a processed message from the queue.
func (s *Storage) Delete(ctx context.Context, id, receipt string) error {
	_, err := s.queue.DeleteMessage(ctx, id, receipt, nil)
	return err
}

// GetTask retrieves a task entity if present.
func (s *Storage) GetTask(ctx context.Context, pk, rk string) (*domain.TaskEntity, error) {
	ent, err := s.taskTable.GetEntity(ctx, pk, rk, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}
	var raw struct {
		PartitionKey       string          `json:"PartitionKey"`
		RowKey             string          `json:"RowKey"`
		Title              string          `json:"Title,omitempty"`
		Notes              string          `json:"Notes,omitempty"`
		Category           string          `json:"Category,omitempty"`
		Order              int             `json:"Order"`
		OrderType          string          `json:"Order@odata.type"`
		Done               bool            `json:"Done"`
		DoneType           string          `json:"Done@odata.type"`
		EventTimestamp     json.RawMessage `json:"EventTimestamp"`
		EventTimestampType string          `json:"EventTimestamp@odata.type"`
	}
	if err := json.Unmarshal(ent.Value, &raw); err != nil {
		return nil, err
	}
	task := domain.TaskEntity{
		Entity:             domain.Entity{PartitionKey: raw.PartitionKey, RowKey: raw.RowKey},
		Title:              raw.Title,
		Notes:              raw.Notes,
		Category:           raw.Category,
		Order:              raw.Order,
		OrderType:          raw.OrderType,
		Done:               raw.Done,
		DoneType:           raw.DoneType,
		EventTimestamp:     parseTimestamp(raw.EventTimestamp),
		EventTimestampType: raw.EventTimestampType,
	}
	return &task, nil
}

// InsertTask adds a new task entity if it does not already exist.
func (s *Storage) InsertTask(ctx context.Context, ent domain.TaskEntity) error {
	payload, err := json.Marshal(ent)
	if err == nil {
		_, err = s.taskTable.AddEntity(ctx, payload, nil)
	}
	return err
}

// UpsertTask creates or replaces a task entity.
func (s *Storage) UpsertTask(ctx context.Context, ent domain.TaskEntity) error {
	payload, err := json.Marshal(ent)
	if err == nil {
		_, err = s.taskTable.UpsertEntity(ctx, payload, nil)
	}
	return err
}

// UpdateTask merges changes into an existing task entity.
func (s *Storage) UpdateTask(ctx context.Context, ent domain.TaskUpdate) error {
	payload, err := json.Marshal(ent)
	if err == nil {
		et := azcore.ETagAny
		_, err = s.taskTable.UpdateEntity(ctx, payload, &aztables.UpdateEntityOptions{IfMatch: &et, UpdateMode: aztables.UpdateModeMerge})
	}
	return err
}

// SetTaskDone marks a task as completed.
func (s *Storage) SetTaskDone(ctx context.Context, pk, rk string) error {
	done := true
	t := domain.EdmBoolean
	ent := domain.TaskUpdate{
		Entity:   domain.Entity{PartitionKey: pk, RowKey: rk},
		Done:     &done,
		DoneType: &t,
	}
	payload, err := json.Marshal(ent)
	if err == nil {
		et := azcore.ETagAny
		_, err = s.taskTable.UpdateEntity(ctx, payload, &aztables.UpdateEntityOptions{IfMatch: &et, UpdateMode: aztables.UpdateModeMerge})
	}
	return err
}

// UpsertUser creates or replaces a user entity.
func (s *Storage) UpsertUser(ctx context.Context, ent domain.UserEntity) error {
	payload, err := json.Marshal(ent)
	if err == nil {
		_, err = s.userTable.UpsertEntity(ctx, payload, nil)
	}
	return err
}

// GetUserSettings retrieves user settings if present.
func (s *Storage) GetUserSettings(ctx context.Context, id string) (*domain.UserSettingsEntity, error) {
	ent, err := s.settingsTable.GetEntity(ctx, id, id, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}
	var raw struct {
		PartitionKey       string          `json:"PartitionKey"`
		RowKey             string          `json:"RowKey"`
		TasksPerCategory   int             `json:"TasksPerCategory"`
		TasksPerCategoryTy string          `json:"TasksPerCategory@odata.type"`
		ShowDoneTasks      bool            `json:"ShowDoneTasks"`
		ShowDoneTasksType  string          `json:"ShowDoneTasks@odata.type"`
		EventTimestamp     json.RawMessage `json:"EventTimestamp"`
		EventTimestampType string          `json:"EventTimestamp@odata.type"`
	}
	if err := json.Unmarshal(ent.Value, &raw); err != nil {
		return nil, err
	}
	sEnt := domain.UserSettingsEntity{
		Entity:               domain.Entity{PartitionKey: raw.PartitionKey, RowKey: raw.RowKey},
		TasksPerCategory:     raw.TasksPerCategory,
		TasksPerCategoryType: raw.TasksPerCategoryTy,
		ShowDoneTasks:        raw.ShowDoneTasks,
		ShowDoneTasksType:    raw.ShowDoneTasksType,
		EventTimestamp:       parseTimestamp(raw.EventTimestamp),
		EventTimestampType:   raw.EventTimestampType,
	}
	return &sEnt, nil
}

func (s *Storage) UpsertUserSettings(ctx context.Context, ent domain.UserSettingsEntity) error {
	payload, err := json.Marshal(ent)
	if err == nil {
		_, err = s.settingsTable.UpsertEntity(ctx, payload, nil)
	}
	return err
}

// InsertUserSettings adds a new user settings entity if it does not already exist.
func (s *Storage) InsertUserSettings(ctx context.Context, ent domain.UserSettingsEntity) error {
	payload, err := json.Marshal(ent)
	if err == nil {
		_, err = s.settingsTable.AddEntity(ctx, payload, nil)
	}
	return err
}

func (s *Storage) UpdateUserSettings(ctx context.Context, ent domain.UserSettingsUpdate) error {
	payload, err := json.Marshal(ent)
	if err == nil {
		et := azcore.ETagAny
		_, err = s.settingsTable.UpdateEntity(ctx, payload, &aztables.UpdateEntityOptions{IfMatch: &et, UpdateMode: aztables.UpdateModeMerge})
	}
	return err
}
