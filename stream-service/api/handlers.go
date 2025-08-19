package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"

	"stream-service/domain"
)

type Storage interface {
	FetchTasks(ctx context.Context, userID string) ([]domain.Task, error)
}

type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}

type updateBroker struct {
	token string

	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

func newUpdateBroker(token string) *updateBroker {
	return &updateBroker{token: token, subs: make(map[chan struct{}]struct{})}
}

func (b *updateBroker) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *updateBroker) unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *updateBroker) notify() {
	b.mu.Lock()
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	b.mu.Unlock()
}

// handleUpdate accepts update events and notifies SSE subscribers.
func (b *updateBroker) handleUpdate(c echo.Context) error {
	authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != b.token {
		return c.NoContent(http.StatusUnauthorized)
	}
	var payload map[string]any
	if err := c.Bind(&payload); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	b.notify()
	return c.NoContent(http.StatusAccepted)
}

// Register wires up stream endpoints on the given Echo instance.
func Register(e *echo.Echo, store Storage, auth Authenticator, token string) {
	broker := newUpdateBroker(token)
	e.GET("/stream", streamTasks(store, auth, broker))
	e.POST("/updates", broker.handleUpdate)
}

func streamTasks(store Storage, auth Authenticator, broker *updateBroker) echo.HandlerFunc {
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
		ch := broker.subscribe()
		defer broker.unsubscribe(ch)
		for {
			tasks, err := store.FetchTasks(ctx, userID)
			if err != nil {
				c.Logger().Error(err)
				return err
			}
			data, err := json.Marshal(tasks)
			if err != nil {
				c.Logger().Error(err)
				return err
			}
			if _, err := c.Response().Write([]byte("data: ")); err != nil {
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
			select {
			case <-ctx.Done():
				return nil
			case <-ch:
				continue
			}
		}
	}
}
