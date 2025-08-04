package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

var (
	subsMu      sync.Mutex
	subscribers = map[string]map[chan Event]struct{}{}
)

func addSubscriber(userID string) chan Event {
	subsMu.Lock()
	defer subsMu.Unlock()
	ch := make(chan Event, 10)
	if subscribers[userID] == nil {
		subscribers[userID] = make(map[chan Event]struct{})
	}
	subscribers[userID][ch] = struct{}{}
	return ch
}

func removeSubscriber(userID string, ch chan Event) {
	subsMu.Lock()
	defer subsMu.Unlock()
	if subs, ok := subscribers[userID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(subscribers, userID)
		}
	}
}

func broadcast(userID string, ev Event) {
	subsMu.Lock()
	subs := subscribers[userID]
	subsMu.Unlock()
	for ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func streamEvents(c echo.Context) error {
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
	c.Response().WriteHeader(http.StatusOK)
	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return c.String(http.StatusInternalServerError, "stream unsupported")
	}
	// Write an initial comment to ensure headers are flushed to the client.
	if _, err := c.Response().Write([]byte(":ok\n\n")); err != nil {
		return nil
	}
	flusher.Flush()

	ch := addSubscriber(userID)
	defer removeSubscriber(userID, ch)
	ctx := c.Request().Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case ev := <-ch:
			data, _ := json.Marshal(ev)
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
		case <-ticker.C:
			// Send a comment as a heartbeat to keep the connection alive.
			if _, err := c.Response().Write([]byte(":keepalive\n\n")); err != nil {
				return nil
			}
			flusher.Flush()
		case <-ctx.Done():
			return nil
		}
	}
}
