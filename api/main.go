package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	svc          *aztables.ServiceClient
	tableClients map[string]*aztables.Client
	jwtJWKS      *keyfunc.JWKS
	jwtAudience  string
	jwtIssuer    string
	taskTable    string
	userTable    string
)

func clientFor(name string) *aztables.Client {
	if c, ok := tableClients[name]; ok {
		return c
	}
	c := svc.NewClient(name)
	tableClients[name] = c
	return c
}

func ensureTable(ctx context.Context, name string) error {
	if _, err := clientFor(name).CreateTable(ctx, nil); err != nil {
		var respErr *azcore.ResponseError
		if !(errors.As(err, &respErr) && respErr.ErrorCode == string(aztables.TableAlreadyExists)) {
			return err
		}
	}
	return nil
}

func main() {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	taskTable = os.Getenv("TASK_EVENTS_TABLE")
	userTable = os.Getenv("USER_EVENTS_TABLE")
	if connStr == "" || taskTable == "" || userTable == "" {
		log.Fatal("missing table storage config")
	}
	var err error
	svc, err = aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		log.Fatalf("service client: %v", err)
	}
	tableClients = make(map[string]*aztables.Client)
	ctx := context.Background()
	if err := ensureTable(ctx, taskTable); err != nil {
		log.Fatalf("ensure table: %v", err)
	}
	if err := ensureTable(ctx, userTable); err != nil {
		log.Fatalf("ensure table: %v", err)
	}

	jwtAudience = os.Getenv("AUTH0_AUDIENCE")
	domain := os.Getenv("AUTH0_DOMAIN")
	if jwtAudience == "" || domain == "" {
		log.Fatal("missing Auth0 config")
	}
	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)
	jwtJWKS, err = keyfunc.Get(jwksURL, keyfunc.Options{})
	if err != nil {
		log.Fatalf("jwks: %v", err)
	}
	jwtIssuer = "https://" + domain + "/"

	allowedOrigins := []string{
		"https://localhost:8080",
		"https://mistervvp:8080",
		"https://mistervvp.local:8080",
		"https://192.168.50.162:8080",
	}
	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		allowedOrigins = []string{}
		for _, o := range strings.Split(origins, ",") {
			o = strings.TrimSpace(strings.TrimSuffix(o, "/"))
			if o != "" {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
	}

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: allowedOrigins,
		AllowHeaders: []string{echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderAccept},
	}))
	e.GET("/api/tasks", getTasks)
	e.POST("/api/events", postEvents)

	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}

func postEvents(c echo.Context) error {
	ctx := c.Request().Context()
	userID, err := userIDFromAuthHeader(c.Request().Header.Get("Authorization"))
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	var events []Event
	if err := c.Bind(&events); err != nil {
		return c.String(http.StatusBadRequest, "invalid body")
	}
	for _, ev := range events {
		table := taskTable
		if ev.EntityType == "users" {
			table = userTable
		}
		data, _ := json.Marshal(ev)

		if table == userTable && ev.Type == "user-registered" {
			ent := map[string]interface{}{
				"PartitionKey": userID,
				"RowKey":       ev.EntityID,
				"Data":         string(data),
			}
			payload, _ := json.Marshal(ent)
			if _, err := clientFor(table).AddEntity(ctx, payload, nil); err != nil {
				var respErr *azcore.ResponseError
				if errors.As(err, &respErr) && respErr.ErrorCode == string(aztables.EntityAlreadyExists) {
					continue
				}
				log.Printf("add entity: %v", err)
			}
			continue
		}

		ent := map[string]interface{}{
			"PartitionKey": userID,
			"RowKey":       ev.ID,
			"Data":         string(data),
		}
		payload, _ := json.Marshal(ent)
		clientFor(table).UpsertEntity(ctx, payload, nil)
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func fetchEvents(ctx context.Context, userID, table string) ([]Event, error) {
	filter := "PartitionKey eq '" + userID + "'"
	pager := clientFor(table).NewListEntitiesPager(&aztables.ListEntitiesOptions{
		Filter: &filter,
	})
	events := make([]Event, 0)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range resp.Entities {
			var ent eventEntity
			if err := json.Unmarshal(e, &ent); err != nil {
				continue
			}
			var ev Event
			if err := json.Unmarshal([]byte(ent.Data), &ev); err == nil {
				events = append(events, ev)
			}
		}
	}
	return events, nil
}

func getTasks(c echo.Context) error {
	ctx := c.Request().Context()
	userID, err := userIDFromAuthHeader(c.Request().Header.Get("Authorization"))
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	events, err := fetchEvents(ctx, userID, taskTable)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	tasks := applyEvents(events)
	return c.JSON(http.StatusOK, tasks)
}
