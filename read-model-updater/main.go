package main

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"read-model-updater/domain"
	"read-model-updater/storage"
)

type queueMessage struct {
	Data struct {
		Event string `json:"event"`
	} `json:"Data"`
}

// Azure Functions custom handlers expect a JSON body even for non-HTTP triggers.
// See https://learn.microsoft.com/en-us/azure/azure-functions/functions-custom-handlers#response-payload
type azFuncResponse struct {
	Outputs map[string]any `json:"Outputs"`
}

func main() {
	if dbg, err := strconv.ParseBool(os.Getenv("DEBUG")); err == nil && dbg {
		log.SetLevel(log.DebugLevel)
	}
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	eventsQueue := os.Getenv("DOMAIN_EVENTS_QUEUE")
	tasksTable := os.Getenv("TASKS_TABLE")
	usersTable := os.Getenv("USERS_TABLE")
	settingsTable := os.Getenv("SETTINGS_TABLE")
	if connStr == "" || eventsQueue == "" || tasksTable == "" || usersTable == "" || settingsTable == "" {
		log.Fatal("missing storage config")
	}

	st, err := storage.New(connStr, eventsQueue, tasksTable, usersTable, settingsTable)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	orch := domain.NewOrchestrator(domain.NewTaskService(st), domain.NewUserService(st))
	redisConn := os.Getenv("REDIS_CONNECTION_STRING")
	if redisConn == "" {
		log.Fatal("missing redis config")
	}
	redisOpts, err := redis.ParseURL(redisConn)
	if err != nil {
		parts := strings.Split(redisConn, ",")
		redisOpts = &redis.Options{Addr: parts[0]}
		for _, p := range parts[1:] {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch strings.ToLower(kv[0]) {
			case "password":
				redisOpts.Password = kv[1]
			case "ssl":
				if strings.ToLower(kv[1]) == "true" {
					redisOpts.TLSConfig = &tls.Config{}
				}
			}
		}
	}
	rc := redis.NewClient(redisOpts)
	taskUpdatesChannel := os.Getenv("TASK_UPDATES_CHANNEL")
	settingsUpdatesChannel := os.Getenv("SETTINGS_UPDATES_CHANNEL")
	if taskUpdatesChannel == "" || settingsUpdatesChannel == "" {
		log.Fatal("missing redis channel config")
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

		ctx := c.Request().Context()
		if err := processEvent(ctx, orch, rc, taskUpdatesChannel, settingsUpdatesChannel, ev, eventPayload); err != nil {
			log.Errorf("Unable to process message, error: %v", err)
			return c.NoContent(http.StatusBadRequest)
		}

		return c.JSON(http.StatusOK, azFuncResponse{Outputs: map[string]any{}})
	}

	e.POST("/", handler)
	e.POST("/domain-events", handler)

	listenAddr := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		listenAddr = ":" + val
	}
	e.Logger.Fatal(e.Start(listenAddr))
}
