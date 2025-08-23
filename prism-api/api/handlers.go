package api

import (
	"context"
	"net/http"

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

// Register wires up all API routes on the provided Echo instance.
func Register(e *echo.Echo, store Storage, auth Authenticator) {
        e.GET("/api/tasks", getTasks(store, auth))
        e.GET("/api/settings", getSettings(store, auth))
        e.POST("/api/commands", postCommands(store, auth))
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
			c.Logger().Error(err)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	}
}
