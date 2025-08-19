package subscription

import (
	"context"
	"encoding/json"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

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
					UserID string `json:"UserId"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					logger.Errorf("unable to parse update: %v", err)
					continue
				}
				if err := rc.Set(ctx, consts.TasksKeyPrefix+ev.UserID, []byte(msg.Payload), 0).Err(); err != nil {
					logger.Errorf("cache update: %v", err)
				}
				broadcast(ev.UserID, []byte(msg.Payload))
			}
		}
		if ctx.Err() != nil {
			return
		}
		logger.Error("pubsub channel closed, reconnecting")
		time.Sleep(time.Second)
	}
}
