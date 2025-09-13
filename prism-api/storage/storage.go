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
	taskTable     *aztables.Client
	settingsTable *aztables.Client
	commandQueue  *azqueue.QueueClient
	jobs          chan queueJob
}

type queueJob struct {
	ctx     context.Context
	message string
	result  chan error
}

// New creates a Storage instance from the given connection string.
func New(connStr, tasksTable, settingsTable, commandQueue string, concurrency int) (*Storage, error) {
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
	s := &Storage{
		taskTable:     tt,
		settingsTable: st,
		commandQueue:  cq,
		jobs:          make(chan queueJob, concurrency),
	}
	for i := 0; i < concurrency; i++ {
		go s.worker()
	}
	return s, nil
}

func (s *Storage) worker() {
	for job := range s.jobs {
		_, err := s.commandQueue.EnqueueMessage(job.ctx, job.message, nil)
		job.result <- err
	}
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
		resCh := make(chan error, 1)
		s.jobs <- queueJob{ctx: ctx, message: string(data), result: resCh}
		if err := <-resCh; err != nil {
			return err
		}
	}
	return nil
}
