package api

import (
        "context"
        "encoding/json"
        "net/http"
        "sync"

        "github.com/labstack/echo/v4"
        "github.com/redis/go-redis/v9"

        "stream-service/domain"
)

type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}

var (
	clients   = map[string]map[chan []byte]struct{}{}
	clientsMu sync.RWMutex
)

// Register wires up stream endpoints on the given Echo instance.
func Register(e *echo.Echo, rc *redis.Client, auth Authenticator, taskChannel, settingsChannel string) {
        go domain.SubscribeUpdates(context.Background(), e.Logger, rc, taskChannel, broadcast)
        go domain.SubscribeUpdates(context.Background(), e.Logger, rc, settingsChannel, broadcast)
        e.GET("/stream", stream(rc, auth))
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

func stream(rc *redis.Client, auth Authenticator) echo.HandlerFunc {
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
                key := domain.TasksKeyPrefix + userID
                data, err := rc.Get(ctx, key).Bytes()
                if err != nil {
                        data = []byte("[]")
                }
                initial := struct {
                        EntityType string          `json:"entityType"`
                        Data       json.RawMessage `json:"data"`
                }{EntityType: "task", Data: data}
                payload, _ := json.Marshal(initial)
                if _, err := c.Response().Write([]byte(domain.SSEDataPrefix)); err != nil {
                        c.Logger().Error(err)
                        return err
                }
                if _, err := c.Response().Write(payload); err != nil {
                        c.Logger().Error(err)
                        return err
                }
                if _, err := c.Response().Write([]byte("\n\n")); err != nil {
                        c.Logger().Error(err)
                        return err
                }
                skey := domain.SettingsKeyPrefix + userID
                sdata, err := rc.Get(ctx, skey).Bytes()
                if err != nil {
                        sdata = []byte("{}")
                }
                sinitial := struct {
                        EntityType string          `json:"entityType"`
                        Data       json.RawMessage `json:"data"`
                }{EntityType: "user-settings", Data: sdata}
                spayload, _ := json.Marshal(sinitial)
                if _, err := c.Response().Write([]byte(domain.SSEDataPrefix)); err != nil {
                        c.Logger().Error(err)
                        return err
                }
                if _, err := c.Response().Write(spayload); err != nil {
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
                                if _, err := c.Response().Write([]byte(domain.SSEDataPrefix)); err != nil {
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
