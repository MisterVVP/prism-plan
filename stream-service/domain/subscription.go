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
			var ev Event
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				logger.Errorf("unable to parse update: %v", err)
				continue
			}
			var payload struct {
				EntityType string `json:"entityType"`
				Data       any    `json:"data"`
			}
			payload.EntityType = ev.EntityType

			switch ev.EntityType {
			case "task":
				tasks := []Task{}
				switch ev.Type {
				case TaskCreated:
					var taskCreatedEvent TaskCreatedEventData
					if err := json.Unmarshal(ev.Data, &taskCreatedEvent); err != nil {
						logger.Errorf("parse task-created: %v", err)
						continue
					}
					tasks = append(tasks, Task{
						ID:       ev.EntityID,
						Title:    taskCreatedEvent.Title,
						Notes:    taskCreatedEvent.Notes,
						Category: taskCreatedEvent.Category,
						Order:    taskCreatedEvent.Order,
					})
				case TaskUpdated:
					var taskUpdatedEvent TaskUpdatedEventData
					if err := json.Unmarshal(ev.Data, &taskUpdatedEvent); err != nil {
						logger.Errorf("parse task-updated: %v", err)
						continue
					}
					newTask := Task{ID: ev.EntityID}
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
					if taskUpdatedEvent.Done != nil {
						newTask.Done = taskUpdatedEvent.Done
					}
					tasks = append(tasks, newTask)
				case TaskCompleted:
					tasks = append(tasks, Task{ID: ev.EntityID, Done: boolPtr(true)})
				case TaskReopened:
					tasks = append(tasks, Task{ID: ev.EntityID, Done: boolPtr(false)})
				default:
					logger.Warnf("Received unknown task event of type %s in %s channel - ignoring it", ev.Type, readModelUpdatesChannel)
					continue
				}
				payload.Data = tasks
			case "user-settings":
				var settingsEvent UserSettingsEventData
				if err := json.Unmarshal(ev.Data, &settingsEvent); err != nil {
					logger.Errorf("parse user-settings: %v", err)
					continue
				}
				settings := UserSettings{TasksPerCategory: settingsEvent.TasksPerCategory, ShowDoneTasks: settingsEvent.ShowDoneTasks}
				payload.Data = settings
			default:
				logger.Warnf("Received unknown entity type %s in %s channel - ignoring it", ev.EntityType, readModelUpdatesChannel)
				continue
			}

			data, err := json.Marshal(payload)
			if err != nil {
				logger.Errorf("marshal payload: %v", err)
				continue
			}

			broadcast(ev.UserID, data)
		}
	}
}
