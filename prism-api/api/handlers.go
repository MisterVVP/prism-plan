package api

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"prism-api/domain"
)

// Storage abstracts persistence for handlers.
type Storage interface {
	FetchTasks(ctx context.Context, userID string) ([]domain.Task, error)
	FetchSettings(ctx context.Context, userID string) (domain.Settings, error)
	EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error
}

// Authenticator is implemented by types able to extract user IDs from headers.
type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}

// Deduper prevents processing of duplicate commands.
type Deduper interface {
	// Add records the idempotency key and returns true if it was newly added.
	Add(ctx context.Context, userID, key string) (bool, error)
	// Remove deletes a previously added key, used when downstream processing fails.
	Remove(ctx context.Context, userID, key string) error
}

// Register wires up all API routes on the provided Echo instance.
func Register(e *echo.Echo, store Storage, auth Authenticator, deduper Deduper) {
	e.GET("/healthz", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.GET("/api/tasks", getTasks(store, auth))
	e.GET("/api/settings", getSettings(store, auth))
	e.POST("/api/commands", postCommands(store, auth, deduper))
}

var (
	lastTimestamp int64
)

func nextTimestamp() int64 {
	for {
		now := time.Now().UnixNano()
		last := atomic.LoadInt64(&lastTimestamp)
		if now <= last {
			now = last + 1
		}
		if atomic.CompareAndSwapInt64(&lastTimestamp, last, now) {
			return now
		}
	}
}

func getTasks(store Storage, auth Authenticator) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		userID, err := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}
		tasks, err := store.FetchTasks(ctx, userID)
		if err != nil {
			c.Logger().Error(err)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, tasks)
	}
}

func getSettings(store Storage, auth Authenticator) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		userID, err := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}
		settings, err := store.FetchSettings(ctx, userID)
		if err != nil {
			c.Logger().Error(err)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, settings)
	}
}

func postCommands(store Storage, auth Authenticator, deduper Deduper) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		userID, err := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}
		var cmds []domain.Command
		if err := c.Bind(&cmds); err != nil {
			return c.String(http.StatusBadRequest, "invalid body")
		}
		keys := make([]string, len(cmds))
		filtered := make([]domain.Command, 0, len(cmds))
		added := make([]string, 0, len(cmds))
		for i := range cmds {
			if cmds[i].IdempotencyKey == "" {
				cmds[i].IdempotencyKey = uuid.NewString()
			}
			cmds[i].ID = cmds[i].IdempotencyKey
			keys[i] = cmds[i].IdempotencyKey
			addedNow, err := deduper.Add(ctx, userID, cmds[i].IdempotencyKey)
			if err != nil {
				c.Logger().Error(err)
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if !addedNow {
				continue
			}
			added = append(added, cmds[i].IdempotencyKey)
			cmds[i].Timestamp = nextTimestamp()
			filtered = append(filtered, cmds[i])
		}
		if len(filtered) == 0 {
			return c.JSON(http.StatusOK, map[string][]string{"idempotencyKeys": keys})
		}
		if err := store.EnqueueCommands(ctx, userID, filtered); err != nil {
			for _, key := range added {
				if remErr := deduper.Remove(ctx, userID, key); remErr != nil {
					c.Logger().Error(remErr)
				}
			}
			c.Logger().Error(err)
			return c.JSON(http.StatusInternalServerError, map[string]any{"idempotencyKeys": keys, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string][]string{"idempotencyKeys": keys})
	}
}
