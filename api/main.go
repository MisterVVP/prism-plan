package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/labstack/echo/v4"
)

type Task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Lane  string `json:"lane"`
	Color string `json:"color"`
	Shape string `json:"shape"`
}

type entity struct {
	aztables.Entity
	Data string `json:"Data"`
}

var tableClient *aztables.Client

func main() {
	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	tableName := os.Getenv("TABLE_NAME")
	if connStr == "" || tableName == "" {
		log.Fatal("missing table storage config")
	}
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		log.Fatalf("service client: %v", err)
	}
	tableClient = svc.NewClient(tableName)
	ctx := context.Background()
	_, _ = tableClient.CreateTable(ctx, nil)

	e := echo.New()
	e.GET("/api/tasks", getTasks)
	e.POST("/api/tasks", postTasks)

	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}
	e.Logger.Fatal(e.Start(":" + port))
}

func getTasks(c echo.Context) error {
	ctx := c.Request().Context()
	pager := tableClient.NewListEntitiesPager(nil)
	var tasks []Task
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		for _, e := range resp.Entities {
			var ent entity
			if err := json.Unmarshal(e, &ent); err != nil {
				continue
			}
			var t Task
			if err := json.Unmarshal([]byte(ent.Data), &t); err == nil {
				tasks = append(tasks, t)
			}
		}
	}
	return c.JSON(http.StatusOK, tasks)
}

func postTasks(c echo.Context) error {
	ctx := c.Request().Context()
	var tasks []Task
	if err := c.Bind(&tasks); err != nil {
		return c.String(http.StatusBadRequest, "invalid body")
	}
	existing := map[string]struct{}{}
	pager := tableClient.NewListEntitiesPager(nil)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		for _, e := range resp.Entities {
			var row struct{ RowKey string }
			if err := json.Unmarshal(e, &row); err == nil {
				existing[row.RowKey] = struct{}{}
			}
		}
	}
	for _, t := range tasks {
		data, _ := json.Marshal(t)
		ent := map[string]interface{}{
			"PartitionKey": "main",
			"RowKey":       t.ID,
			"Data":         string(data),
		}
		payload, _ := json.Marshal(ent)
		if _, err := tableClient.AddEntity(ctx, payload, nil); err != nil {
			tableClient.UpsertEntity(ctx, payload, nil)
		}
		delete(existing, t.ID)
	}
	for id := range existing {
		tableClient.DeleteEntity(ctx, "main", id, nil)
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
