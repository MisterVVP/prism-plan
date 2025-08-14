package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"read-model-updater/domain"
	"read-model-updater/storage"
)

func main() {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	eventsQueue := os.Getenv("DOMAIN_EVENTS_QUEUE")
	tasksTable := os.Getenv("TASKS_TABLE")
	usersTable := os.Getenv("USERS_TABLE")
	if connStr == "" || eventsQueue == "" || tasksTable == "" || usersTable == "" {
		log.Fatal("missing storage config")
	}

	st, err := storage.New(connStr, eventsQueue, tasksTable, usersTable)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	e := echo.New()
	handler := func(c echo.Context) error {
		var payload struct {
			Data struct {
				Msg string `json:"msg"`
			} `json:"Data"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		var ev domain.Event
		if err := json.Unmarshal([]byte(payload.Data.Msg), &ev); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		domain.Apply(c.Request().Context(), st, ev)
		return c.NoContent(http.StatusOK)
	}

	e.POST("/", handler)
	e.POST("/domain-events", handler)

	listenAddr := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		listenAddr = ":" + val
	}
	e.Logger.Fatal(e.Start(listenAddr))
}
