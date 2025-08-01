package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	tableClient *aztables.Client
	jwtJWKS     *keyfunc.JWKS
	jwtAudience string
	jwtIssuer   string
)

func main() {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	tableName := os.Getenv("TASK_EVENTS_TABLE")
	if connStr == "" || tableName == "" {
		log.Fatal("missing table storage config")
	}
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		log.Fatalf("service client: %v", err)
	}
	tableClient = svc.NewClient(tableName)
	ctx := context.Background()
	if _, err = tableClient.CreateTable(ctx, nil); err != nil {
		var respErr *azcore.ResponseError
		if !(errors.As(err, &respErr) && respErr.ErrorCode == string(aztables.TableAlreadyExists)) {
			log.Fatalf("create table: %v", err)
		}
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

	e := echo.New()
	e.Use(middleware.CORS())
	e.GET("/api/events", getEvents)
	e.GET("/api/tasks", getTasks)
	e.POST("/api/events", postEvents)
	e.POST("/api/user", postUser)

	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}

func getEvents(c echo.Context) error {
	ctx := c.Request().Context()
	userID, err := userIDFromAuthHeader(c.Request().Header.Get("Authorization"))
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	events, err := fetchEvents(ctx, userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, events)
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
		data, _ := json.Marshal(ev)
		ent := map[string]interface{}{
			"PartitionKey": userID,
			"RowKey":       ev.ID,
			"Data":         string(data),
		}
		payload, _ := json.Marshal(ent)
		tableClient.UpsertEntity(ctx, payload, nil)
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func fetchEvents(ctx context.Context, userID string) ([]Event, error) {
	filter := "PartitionKey eq '" + userID + "'"
	pager := tableClient.NewListEntitiesPager(&aztables.ListEntitiesOptions{
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
	events, err := fetchEvents(ctx, userID)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	tasks := applyEvents(events)
	return c.JSON(http.StatusOK, tasks)
}
