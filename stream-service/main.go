package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"

	"stream-service/api"
	"stream-service/storage"
)

func main() {
	if dbg, err := strconv.ParseBool(os.Getenv("DEBUG")); err == nil && dbg {
		log.SetLevel(log.DebugLevel)
	}
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	tasksTableName := os.Getenv("TASKS_TABLE")
	if connStr == "" || tasksTableName == "" {
		log.Fatal("missing storage config")
	}
	store, err := storage.New(connStr, tasksTableName)
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

	api.Register(e, store, auth)

	listenAddr := ":9000"
	if val, ok := os.LookupEnv("STREAM_SERVICE_PORT"); ok {
		listenAddr = ":" + val
	}

	e.Logger.Fatal(e.Start(listenAddr))
}
