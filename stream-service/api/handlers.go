package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"stream-service/domain"
	"stream-service/internal/consts"
	"stream-service/subscription"
)

type Storage interface {
	FetchTasks(ctx context.Context, userID string) ([]domain.Task, error)
}

type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}

var (
	clients   = map[string]map[chan []byte]struct{}{}
	clientsMu sync.RWMutex
)

// Register wires up stream endpoints on the given Echo instance.
func Register(e *echo.Echo, store Storage, rc *redis.Client, auth Authenticator, readModelUpdatesChannel string) {
	go subscription.SubscribeUpdates(context.Background(), e.Logger, rc, store, readModelUpdatesChannel, broadcast)
	e.GET("/stream", streamTasks(store, rc, auth))
}

func addClient(userID string, ch chan []byte) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if clients[userID] == nil {
		clients[userID] = make(map[chan []byte]struct{})
	}
	clients[userID][ch] = struct{}{}
}

func removeClient(userID string, ch chan []byte) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if m, ok := clients[userID]; ok {
		delete(m, ch)
		if len(m) == 0 {
			delete(clients, userID)
		}
	}
}

func broadcast(userID string, msg []byte) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	for ch := range clients[userID] {
		select {
		case ch <- msg:
		default:
		}
	}
}

func streamTasks(store Storage, rc *redis.Client, auth Authenticator) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.QueryParam("token")
		authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
		if authHeader == "" && token != "" {
			authHeader = "Bearer " + token
		}
		userID, err := auth.UserIDFromAuthHeader(authHeader)
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}
		c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
		c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
		c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
		c.Response().Header().Set("X-Accel-Buffering", "no")
		flusher, ok := c.Response().Writer.(http.Flusher)
		if !ok {
			return c.String(http.StatusInternalServerError, "stream unsupported")
		}
		ctx := c.Request().Context()
		key := consts.TasksKeyPrefix + userID
		data, err := rc.Get(ctx, key).Bytes()
		if err != nil {
			tasks, err := store.FetchTasks(ctx, userID)
			if err != nil {
				c.Logger().Error(err)
				return err
			}
			data, err = json.Marshal(tasks)
			if err != nil {
				c.Logger().Error(err)
				return err
			}
			if err := rc.Set(ctx, key, data, 0).Err(); err != nil {
				c.Logger().Error(err)
			}
		}
		if _, err := c.Response().Write([]byte(consts.SSEDataPrefix)); err != nil {
			c.Logger().Error(err)
			return err
		}
		if _, err := c.Response().Write(data); err != nil {
			c.Logger().Error(err)
			return err
		}
		if _, err := c.Response().Write([]byte("\n\n")); err != nil {
			c.Logger().Error(err)
			return err
		}
		flusher.Flush()

		ch := make(chan []byte, 1)
		addClient(userID, ch)
		defer removeClient(userID, ch)

		for {
			select {
			case <-ctx.Done():
				return nil
			case msg := <-ch:
				if _, err := c.Response().Write([]byte(consts.SSEDataPrefix)); err != nil {
					c.Logger().Error(err)
					return err
				}
				if _, err := c.Response().Write(msg); err != nil {
					c.Logger().Error(err)
					return err
				}
				if _, err := c.Response().Write([]byte("\n\n")); err != nil {
					c.Logger().Error(err)
					return err
				}
				flusher.Flush()
			}
		}
	}
}
