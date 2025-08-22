package domain

import (
	"context"
	"encoding/json"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// SubscribeUpdates listens for read model updates and broadcasts tasks to clients.
func SubscribeUpdates(
	ctx context.Context,
	logger echo.Logger,
	rc *redis.Client,
	readModelUpdatesChannel string,
	broadcast func(userID string, data []byte),
) {
	sub := rc.Subscribe(ctx, readModelUpdatesChannel)
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				logger.Error("subscription channel closed")
				return
			}
			var taskEvent TaskEvent
			if err := json.Unmarshal([]byte(msg.Payload), &taskEvent); err != nil {
				logger.Errorf("unable to parse update: %v", err)
				continue
			}

			tasks := []Task{}

			switch taskEvent.Type {
			case TaskCreated:
				var taskCreatedEvent TaskCreatedEventData
				if err := json.Unmarshal(taskEvent.Data, &taskCreatedEvent); err != nil {
					logger.Errorf("parse task-created: %v", err)
					continue
				}
				tasks = append(tasks, Task{
					ID:       taskEvent.EntityID,
					Title:    taskCreatedEvent.Title,
					Notes:    taskCreatedEvent.Notes,
					Category: taskCreatedEvent.Category,
					Order:    taskCreatedEvent.Order,
				})
			case TaskUpdated:
				var taskUpdatedEvent TaskUpdatedEventData
				if err := json.Unmarshal(taskEvent.Data, &taskUpdatedEvent); err != nil {
					logger.Errorf("parse task-updated: %v", err)
					continue
				}
				newTask := Task{ID: taskEvent.EntityID}
				if taskUpdatedEvent.Title != nil {
					newTask.Title = *taskUpdatedEvent.Title
				}
				if taskUpdatedEvent.Notes != nil {
					newTask.Notes = *taskUpdatedEvent.Notes
				}
				if taskUpdatedEvent.Category != nil {
					newTask.Category = *taskUpdatedEvent.Category
				}
				if taskUpdatedEvent.Order != nil {
					newTask.Order = *taskUpdatedEvent.Order
				}
				tasks = append(tasks, newTask)
			case TaskCompleted:
				tasks = append(tasks, Task{
					ID:   taskEvent.EntityID,
					Done: true,
				})
			default:
				logger.Warnf("Received unknown task event of type %s in %s channel - ignoring it", taskEvent.Type, readModelUpdatesChannel)
				continue
			}

			data, err := json.Marshal(tasks)
			if err != nil {
				logger.Errorf("marshal tasks: %v", err)
				continue
			}

			broadcast(taskEvent.UserID, data)
		}
	}
}
