package storage

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"

	"stream-service/domain"
)

// Storage provides access to underlying persistence mechanisms.
type Storage struct {
	taskTable *aztables.Client
}

// New creates a Storage instance from the given connection string.
func New(connStr, tasksTable string) (*Storage, error) {
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		return nil, err
	}
	tt := svc.NewClient(tasksTable)
	return &Storage{taskTable: tt}, nil
}

type taskEntity struct {
	aztables.Entity
	Title    string `json:"Title"`
	Notes    string `json:"Notes"`
	Category string `json:"Category"`
	Order    int    `json:"Order"`
	Done     bool   `json:"Done"`
}

// FetchTasks retrieves all tasks for the provided user.
func (s *Storage) FetchTasks(ctx context.Context, userID string) ([]domain.Task, error) {
	filter := "PartitionKey eq '" + userID + "'"
	pager := s.taskTable.NewListEntitiesPager(&aztables.ListEntitiesOptions{Filter: &filter})
	tasks := []domain.Task{}
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range resp.Entities {
			var ent taskEntity
			if err := json.Unmarshal(e, &ent); err != nil {
				return nil, err
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
	}
	return tasks, nil
}
