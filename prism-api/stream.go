package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func streamTasks(c echo.Context) error {
	token := c.QueryParam("token")
	auth := c.Request().Header.Get("Authorization")
	if auth == "" && token != "" {
		auth = "Bearer " + token
	}
	userID, err := userIDFromAuthHeader(auth)
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
		tasks, err := fetchTasks(ctx, userID)
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
