package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"

	"read-model-updater/domain"
	"read-model-updater/storage"
)

type queueMessage struct {
	Data struct {
		Event string `json:"event"`
	} `json:"Data"`
}

func main() {
	if dbg, err := strconv.ParseBool(os.Getenv("DEBUG")); err == nil && dbg {
		log.SetLevel(log.DebugLevel)
	}
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
		var msg queueMessage
		if err := c.Bind(&msg); err != nil {
			log.Errorf("Unable to parse message JSON, error: %v", err)
			return c.NoContent(http.StatusBadRequest)
		}

		eventPayload := msg.Data.Event
		var unquoted string
		if err := json.Unmarshal([]byte(eventPayload), &unquoted); err == nil {
			eventPayload = unquoted
		} else {
			log.Debugf("unable to unquote event payload: %v", err)
		}

		var ev domain.Event
		if err := json.Unmarshal([]byte(eventPayload), &ev); err != nil {
			log.Errorf("Unable to parse message JSON, error: %v", err)
			return c.NoContent(http.StatusBadRequest)
		}

		if err := domain.Apply(c.Request().Context(), st, ev); err != nil {
			log.Errorf("Unable to process message, error: %v", err)
			return c.NoContent(http.StatusBadRequest)
		}

		// Azure Functions custom handlers expect a JSON body even for
		// non-HTTP triggers. Returning an empty object signals
		// successful processing and allows the runtime to delete the
		// queue message.
		return c.JSON(http.StatusOK, map[string]any{"outputs": map[string]any{}})
	}

	e.POST("/", handler)
	e.POST("/domain-events", handler)

	listenAddr := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		listenAddr = ":" + val
	}
	e.Logger.Fatal(e.Start(listenAddr))
}
