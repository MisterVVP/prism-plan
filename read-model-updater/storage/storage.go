package storage

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"

	"read-model-updater/domain"
)

// Storage wraps Azure clients used by the service.
type Storage struct {
	queue     *azqueue.QueueClient
	taskTable *aztables.Client
	userTable *aztables.Client
}

// New creates a Storage from connection parameters.
func New(connStr, eventsQueue, tasksTable, usersTable string) (*Storage, error) {
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
	return &Storage{queue: queue, taskTable: taskClient, userTable: userClient}, nil
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
