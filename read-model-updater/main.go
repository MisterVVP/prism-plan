package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
)

type Event struct {
	ID         string          `json:"id"`
	EntityID   string          `json:"entityId"`
	EntityType string          `json:"entityType"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	Time       int64           `json:"time"`
	UserID     string          `json:"userId"`
}

func main() {
	log.Println("Read-Model Updater Service starting")

	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	eventsQueue := os.Getenv("DOMAIN_EVENTS_QUEUE")
	tasksTable := os.Getenv("TASKS_TABLE")
	usersTable := os.Getenv("USERS_TABLE")
	if connStr == "" || eventsQueue == "" || tasksTable == "" || usersTable == "" {
		log.Fatal("missing storage config")
	}

	queue, err := azqueue.NewQueueClientFromConnectionString(connStr, eventsQueue, nil)
	if err != nil {
		log.Fatalf("queue client: %v", err)
	}

	tSvc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		log.Fatalf("table service: %v", err)
	}
	taskClient := tSvc.NewClient(tasksTable)
	userClient := tSvc.NewClient(usersTable)

	ctx := context.Background()
	for {
		resp, err := queue.DequeueMessage(ctx, nil)
		if err != nil {
			log.Printf("receive: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if len(resp.Messages) == 0 {
			time.Sleep(time.Second)
			continue
		}
		msg := resp.Messages[0]
		var ev Event
		if err := json.Unmarshal([]byte(*msg.MessageText), &ev); err == nil {
			apply(ctx, taskClient, userClient, ev)
		}
		queue.DeleteMessage(ctx, *msg.MessageID, *msg.PopReceipt, nil)
	}
}

func apply(ctx context.Context, taskTable, userTable *aztables.Client, ev Event) {
	pk := ev.UserID
	rk := ev.EntityID
	switch ev.Type {
	case "task-created":
		var t map[string]any
		if err := json.Unmarshal(ev.Data, &t); err != nil {
			return
		}
		ent := map[string]any{
			"PartitionKey": pk,
			"RowKey":       rk,
			"Title":        t["title"],
			"Notes":        t["notes"],
			"Category":     t["category"],
			"Order":        t["order"],
			"Done":         false,
		}
		payload, _ := json.Marshal(ent)
		taskTable.UpsertEntity(ctx, payload, nil)
	case "task-updated":
		var changes map[string]interface{}
		if err := json.Unmarshal(ev.Data, &changes); err != nil {
			return
		}
		updates := map[string]any{
			"PartitionKey": pk,
			"RowKey":       rk,
		}
		for k, v := range changes {
			if k == "" {
				continue
			}
			capKey := strings.ToUpper(k[:1]) + k[1:]
			updates[capKey] = v
		}
		payload, _ := json.Marshal(updates)
		et := azcore.ETagAny
		taskTable.UpdateEntity(ctx, payload, &aztables.UpdateEntityOptions{IfMatch: &et, UpdateMode: aztables.UpdateModeMerge})
	case "task-completed":
		ent := map[string]any{
			"PartitionKey": pk,
			"RowKey":       rk,
			"Done":         true,
		}
		payload, _ := json.Marshal(ent)
		et2 := azcore.ETagAny
		taskTable.UpdateEntity(ctx, payload, &aztables.UpdateEntityOptions{IfMatch: &et2, UpdateMode: aztables.UpdateModeMerge})
	case "user-created":
		var u map[string]any
		if err := json.Unmarshal(ev.Data, &u); err != nil {
			return
		}
		ent := map[string]any{
			"PartitionKey": rk,
			"RowKey":       rk,
			"Name":         u["name"],
			"Email":        u["email"],
		}
		payload, _ := json.Marshal(ent)
		userTable.UpsertEntity(ctx, payload, nil)
	case "user-logged-in":
		log.Printf("user logged in: %s", rk)
	case "user-logged-out":
		log.Printf("user logged out: %s", rk)
	}
}
