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
}

// Authenticator extracts user IDs from authorization headers.
type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}

// Register wires up SSE route.
func Register(e *echo.Echo, store Storage, auth Authenticator) {
	e.GET("/stream", streamTasks(store, auth))
}

func streamTasks(store Storage, auth Authenticator) echo.HandlerFunc {
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
