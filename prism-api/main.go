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
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
	"github.com/MicahParks/keyfunc"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	taskTable    *aztables.Client
	commandQueue *azqueue.QueueClient
	jwtJWKS      *keyfunc.JWKS
	jwtAudience  string
	jwtIssuer    string
)

func ensureTable(ctx context.Context, svc *aztables.ServiceClient, name string) (*aztables.Client, error) {
	c := svc.NewClient(name)
	if _, err := c.CreateTable(ctx, nil); err != nil {
		var respErr *azcore.ResponseError
		if !(errors.As(err, &respErr) && respErr.ErrorCode == string(aztables.TableAlreadyExists)) {
			return nil, err
		}
	}
	return c, nil
}

func main() {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	tasksTableName := os.Getenv("TASKS_TABLE")
	commandQueueName := os.Getenv("COMMAND_QUEUE")
	if connStr == "" || tasksTableName == "" || commandQueueName == "" {
		log.Fatal("missing storage config")
	}

	var err error
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		log.Fatalf("table service: %v", err)
	}
	ctx := context.Background()
	taskTable, err = ensureTable(ctx, svc, tasksTableName)
	if err != nil {
		log.Fatalf("ensure table: %v", err)
	}

	commandQueue, err = azqueue.NewQueueClientFromConnectionString(connStr, commandQueueName, nil)
	if err != nil {
		log.Fatalf("queue service: %v", err)
	}
	commandQueue.Create(ctx, nil)

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
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	e.GET("/api/tasks", getTasks)
	e.POST("/api/commands", postCommands)

	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}

type Command struct {
	ID         string          `json:"id"`
	EntityID   string          `json:"entityId"`
	EntityType string          `json:"entityType"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data,omitempty"`
}

type commandEnvelope struct {
	UserID  string  `json:"userId"`
	Command Command `json:"command"`
}

func postCommands(c echo.Context) error {
	ctx := c.Request().Context()
	userID, err := userIDFromAuthHeader(c.Request().Header.Get("Authorization"))
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	var cmds []Command
	if err := c.Bind(&cmds); err != nil {
		return c.String(http.StatusBadRequest, "invalid body")
	}
	for _, cmd := range cmds {
		env := commandEnvelope{UserID: userID, Command: cmd}
		data, _ := json.Marshal(env)
		commandQueue.EnqueueMessage(ctx, string(data), nil)
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

type taskEntity struct {
	aztables.Entity
	Title    string `json:"title"`
	Notes    string `json:"notes"`
	Category string `json:"category"`
	Order    int    `json:"order"`
	Done     bool   `json:"done"`
}

func getTasks(c echo.Context) error {
	ctx := c.Request().Context()
	userID, err := userIDFromAuthHeader(c.Request().Header.Get("Authorization"))
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	filter := "PartitionKey eq '" + userID + "'"
	pager := taskTable.NewListEntitiesPager(&aztables.ListEntitiesOptions{Filter: &filter})
	tasks := []Task{}
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		for _, e := range resp.Entities {
			var ent taskEntity
			if err := json.Unmarshal(e, &ent); err == nil {
				tasks = append(tasks, Task{
					ID:       ent.RowKey,
					Title:    ent.Title,
					Notes:    ent.Notes,
					Category: ent.Category,
					Order:    ent.Order,
					Done:     ent.Done,
				})
			}
		}
	}
	return c.JSON(http.StatusOK, tasks)
}
