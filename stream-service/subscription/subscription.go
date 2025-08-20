package subscription

import (
	"context"
	"encoding/json"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"stream-service/domain"
	"stream-service/internal/consts"
)

// SubscribeUpdates listens for read model updates and broadcasts tasks to clients.
func SubscribeUpdates(
	ctx context.Context,
	logger echo.Logger,
	rc *redis.Client,
	readModelUpdatesChannel string,
	broadcast func(userID string, data []byte),
) {
	for {
		sub := rc.Subscribe(ctx, readModelUpdatesChannel)
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					break
				}
				var ev struct {
					UserID   string          `json:"UserId"`
					EntityID string          `json:"EntityId"`
					Type     string          `json:"Type"`
					Data     json.RawMessage `json:"Data"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					logger.Errorf("unable to parse update: %v", err)
					continue
				}

                               key := consts.TasksKeyPrefix + ev.UserID
                               tasks := []domain.Task{}
                               if b, err := rc.Get(ctx, key).Bytes(); err == nil {
                                       if err := json.Unmarshal(b, &tasks); err != nil {
                                               logger.Errorf("unmarshal cache: %v", err)
                                               tasks = nil
                                       }
                               }
                               if tasks == nil {
                                       tasks = []domain.Task{}
                               }

				switch ev.Type {
				case "task-created":
					var d struct {
						Title    string `json:"title"`
						Notes    string `json:"notes"`
						Category string `json:"category"`
						Order    int    `json:"order"`
					}
					if err := json.Unmarshal(ev.Data, &d); err != nil {
						logger.Errorf("parse task-created: %v", err)
						continue
					}
					tasks = append(tasks, domain.Task{
						ID:       ev.EntityID,
						Title:    d.Title,
						Notes:    d.Notes,
						Category: d.Category,
						Order:    d.Order,
					})
				case "task-updated":
					var d struct {
						Title    *string `json:"title"`
						Notes    *string `json:"notes"`
						Category *string `json:"category"`
						Order    *int    `json:"order"`
					}
					if err := json.Unmarshal(ev.Data, &d); err != nil {
						logger.Errorf("parse task-updated: %v", err)
						continue
					}
					for i := range tasks {
						if tasks[i].ID == ev.EntityID {
							if d.Title != nil {
								tasks[i].Title = *d.Title
							}
							if d.Notes != nil {
								tasks[i].Notes = *d.Notes
							}
							if d.Category != nil {
								tasks[i].Category = *d.Category
							}
							if d.Order != nil {
								tasks[i].Order = *d.Order
							}
							break
						}
					}
				case "task-completed":
					for i := range tasks {
						if tasks[i].ID == ev.EntityID {
							tasks[i].Done = true
							break
						}
					}
				default:
					// ignore other events
					continue
				}

				data, err := json.Marshal(tasks)
				if err != nil {
					logger.Errorf("marshal tasks: %v", err)
					continue
				}
				if err := rc.Set(ctx, key, data, 0).Err(); err != nil {
					logger.Errorf("cache update: %v", err)
				}
				broadcast(ev.UserID, data)
			}
		}
		if ctx.Err() != nil {
			return
		}
		logger.Error("pubsub channel closed, reconnecting")
		time.Sleep(time.Second)
	}
}
