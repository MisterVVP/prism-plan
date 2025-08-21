package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// SubscribeUpdates listens for read model updates and broadcasts tasks to clients.
func SubscribeUpdates(
	ctx context.Context,
	logger echo.Logger,
	rc *redis.Client,
	readModelUpdatesChannel string,
	cacheExpiration time.Duration,
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

			key := TasksKeyPrefix + taskEvent.UserID
			tasks := []Task{}
			if b, err := rc.Get(ctx, key).Bytes(); err == nil {
				if err := json.Unmarshal(b, &tasks); err != nil {
					logger.Errorf("unmarshal cache: %v", err)
					tasks = nil
				}
			}
			if tasks == nil {
				tasks = []Task{}
			}

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
				for i := range tasks {
					if tasks[i].ID == taskEvent.EntityID {
						if taskUpdatedEvent.Title != nil {
							tasks[i].Title = *taskUpdatedEvent.Title
						}
						if taskUpdatedEvent.Notes != nil {
							tasks[i].Notes = *taskUpdatedEvent.Notes
						}
						if taskUpdatedEvent.Category != nil {
							tasks[i].Category = *taskUpdatedEvent.Category
						}
						if taskUpdatedEvent.Order != nil {
							tasks[i].Order = *taskUpdatedEvent.Order
						}
						break
					}
				}
			case TaskCompleted:
				for i := range tasks {
					if tasks[i].ID == taskEvent.EntityID {
						tasks[i].Done = true
						break
					}
				}
			default:
				logger.Warnf("Received unknown task event of type %s in %s channel - ignoring it", taskEvent.Type, readModelUpdatesChannel)
				continue
			}

			data, err := json.Marshal(tasks)
			if err != nil {
				logger.Errorf("marshal tasks: %v", err)
				continue
			}
			if err := rc.Set(ctx, key, data, cacheExpiration).Err(); err != nil {
				logger.Errorf("cache update: %v", err)
			}
			broadcast(taskEvent.UserID, data)
		}
	}
}
