package main

import (
	"fmt"
	"log"
	"os"

	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	prismauth "prism-api/api"
	"prism-api/storage"

	"stream-service/api"
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
	auth := prismauth.NewAuth(jwks, jwtAudience, "https://"+domain+"/")

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	api.Register(e, store, auth)

	listenAddr := ":80"
	if val, ok := os.LookupEnv("STREAM_SERVICE_PORT"); ok {
		listenAddr = ":" + val
	}

	e.Logger.Fatal(e.Start(listenAddr))
}
