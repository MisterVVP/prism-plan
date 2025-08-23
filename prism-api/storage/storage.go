package storage

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"

	"prism-api/domain"
)

// Storage provides access to underlying persistence mechanisms.
type Storage struct {
        taskTable    *aztables.Client
        settingsTable *aztables.Client
        commandQueue *azqueue.QueueClient
}

// New creates a Storage instance from the given connection string.
func New(connStr, tasksTable, settingsTable, commandQueue string) (*Storage, error) {
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		return nil, err
	}
        tt := svc.NewClient(tasksTable)
        st := svc.NewClient(settingsTable)
        cq, err := azqueue.NewQueueClientFromConnectionString(connStr, commandQueue, nil)
	if err != nil {
		return nil, err
	}
        return &Storage{taskTable: tt, settingsTable: st, commandQueue: cq}, nil
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

func (s *Storage) FetchSettings(ctx context.Context, userID string) (domain.Settings, error) {
        var settings domain.Settings
        ent, err := s.settingsTable.GetEntity(ctx, userID, userID, nil)
        if err != nil {
                return settings, err
        }
        if err := json.Unmarshal(ent.Value, &settings); err != nil {
                return settings, err
        }
        return settings, nil
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
