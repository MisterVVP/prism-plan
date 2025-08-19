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

// Storage fetches tasks for a user.
type Storage interface {
	FetchTasks(ctx context.Context, userID string) ([]domain.Task, error)
}

// SubscribeUpdates listens for read model updates and broadcasts tasks to clients.
func SubscribeUpdates(
	ctx context.Context,
	logger echo.Logger,
	rc *redis.Client,
	store Storage,
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
					UserID string `json:"UserId"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					logger.Errorf("unable to parse update: %v", err)
					continue
				}
				tasks, err := store.FetchTasks(ctx, ev.UserID)
				if err != nil {
					logger.Errorf("fetch tasks: %v", err)
					continue
				}
				data, err := json.Marshal(tasks)
				if err != nil {
					logger.Errorf("marshal tasks: %v", err)
					continue
				}
				if err := rc.Set(ctx, consts.TasksKeyPrefix+ev.UserID, data, 0).Err(); err != nil {
					logger.Errorf("cache tasks: %v", err)
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
