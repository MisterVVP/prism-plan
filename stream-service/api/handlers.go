package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

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
	e.GET("/healthz", healthz(rc))
}

func healthz(rc *redis.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		redisResp := rc.Ping(c.Request().Context())
		if redisErr := redisResp.Err(); redisErr != nil {
			c.Logger().Errorf("Service unhealthy - redis is unavailable: %v", redisErr)
			return c.String(http.StatusInternalServerError, "Service unavailable")
		}
		return c.NoContent(http.StatusOK)
	}
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
		// Auth
		token := c.QueryParam("token")
		authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
		if authHeader == "" && token != "" {
			authHeader = "Bearer " + token
		}
		userID, err := auth.UserIDFromAuthHeader(authHeader)
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}

		// SSE headers
		res := c.Response()
		res.Header().Set(echo.HeaderContentType, "text/event-stream")
		res.Header().Set(echo.HeaderCacheControl, "no-cache")
		res.Header().Set(echo.HeaderConnection, "keep-alive")
		res.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := res.Writer.(http.Flusher)
		if !ok {
			return c.String(http.StatusInternalServerError, "stream unsupported")
		}

		ctx := c.Request().Context()

		type initialMsg struct {
			EntityType string          `json:"entityType"`
			Data       json.RawMessage `json:"data"`
		}

		// Helpers
		writeSSE := func(payload []byte) error {
			if _, err := res.Write([]byte(domain.SSEDataPrefix)); err != nil {
				return err
			}
			if _, err := res.Write(payload); err != nil {
				return err
			}
			if _, err := res.Write([]byte("\n\n")); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		}

		writeKeepAlive := func() error {
			if _, err := res.Write([]byte(": keep-alive\n\n")); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		}

		getOrDefault := func(key string, def []byte) []byte {
			b, err := rc.Get(ctx, key).Bytes()
			if err != nil {
				return def
			}
			return b
		}

		// Initial payloads
		if payload, _ := json.Marshal(initialMsg{
			EntityType: "task",
			Data:       getOrDefault(domain.TasksKeyPrefix+userID, []byte("[]")),
		}); true {
			if err := writeSSE(payload); err != nil {
				c.Logger().Errorf("stream write: %v", err)
				return nil
			}
		}

		if payload, _ := json.Marshal(initialMsg{
			EntityType: "user-settings",
			Data:       getOrDefault(domain.SettingsKeyPrefix+userID, []byte("{}")),
		}); true {
			if err := writeSSE(payload); err != nil {
				c.Logger().Errorf("stream write: %v", err)
				return nil
			}
		}

		ch := make(chan []byte, 1)
		addClient(userID, ch)
		defer func() {
			removeClient(userID, ch)
			close(ch)
		}()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case msg := <-ch:
				if err := writeSSE(msg); err != nil {
					c.Logger().Errorf("stream write: %v", err)
					return nil
				}
			case <-ticker.C:
				if err := writeKeepAlive(); err != nil {
					c.Logger().Errorf("stream write: %v", err)
					return nil
				}
			}
		}
	}
}
