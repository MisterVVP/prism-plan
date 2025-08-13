package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"prism-api/api"
	"prism-api/storage"
)

func main() {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	tasksTableName := os.Getenv("TASKS_TABLE")
	commandQueueName := os.Getenv("COMMAND_QUEUE")
	if connStr == "" || tasksTableName == "" || commandQueueName == "" {
		log.Fatal("missing storage config")
	}
	store, err := storage.New(connStr, tasksTableName, commandQueueName)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	jwtAudience := os.Getenv("AUTH0_AUDIENCE")
	domain := os.Getenv("AUTH0_DOMAIN")
	if jwtAudience == "" || domain == "" {
		log.Fatal("missing Auth0 config")
	}
	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{})
	if err != nil {
		log.Fatalf("jwks: %v", err)
	}
	auth := api.NewAuth(jwks, jwtAudience, "https://"+domain+"/")

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	e.GET("/stream", streamTasks(store, auth))

	listenAddr := ":80"
	if val, ok := os.LookupEnv("STREAM_SERVICE_PORT"); ok {
		listenAddr = ":" + val
	}

	e.Logger.Fatal(e.Start(listenAddr))
}

func streamTasks(store *storage.Storage, auth *api.Auth) echo.HandlerFunc {
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
