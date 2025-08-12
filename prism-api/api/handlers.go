package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"prism-api/domain"
)

// Storage abstracts persistence for handlers.
type Storage interface {
	FetchTasks(ctx context.Context, userID string) ([]domain.Task, error)
	EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error
}

// Authenticator is implemented by types able to extract user IDs from headers.
type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}

// Register wires up all API routes on the provided Echo instance.
func Register(e *echo.Echo, store Storage, auth Authenticator) {
	e.GET("/api/tasks", getTasks(store, auth))
	e.POST("/api/commands", postCommands(store, auth))
	e.GET("/api/stream", streamTasks(store, auth))
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
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, tasks)
	}
}

func postCommands(store Storage, auth Authenticator) echo.HandlerFunc {
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
		if err := store.EnqueueCommands(ctx, userID, cmds); err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	}
}

func streamTasks(store Storage, auth Authenticator) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.QueryParam("token")
		authHeader := c.Request().Header.Get("Authorization")
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
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			tasks, err := store.FetchTasks(ctx, userID)
			if err == nil {
				data, _ := json.Marshal(tasks)
				if _, err := c.Response().Write([]byte("data: ")); err != nil {
					return nil
				}
				if _, err := c.Response().Write(data); err != nil {
					return nil
				}
				if _, err := c.Response().Write([]byte("\n\n")); err != nil {
					return nil
				}
				flusher.Flush()
			}
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				continue
			}
		}
	}
}
