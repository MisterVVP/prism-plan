package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
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
	if connStr == "" || eventsQueue == "" || tasksTable == "" {
		log.Fatal("missing storage config")
	}

	queue, err := azqueue.NewQueueClientFromConnectionString(connStr, eventsQueue, nil)
	if err != nil {
		log.Fatalf("queue client: %v", err)
	}
	queue.Create(context.Background(), nil)

	tSvc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		log.Fatalf("table service: %v", err)
	}
	table := tSvc.NewClient(tasksTable)
	table.CreateTable(context.Background(), nil)

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
			apply(ctx, table, ev)
		}
		queue.DeleteMessage(ctx, *msg.MessageID, *msg.PopReceipt, nil)
	}
}

func apply(ctx context.Context, table *aztables.Client, ev Event) {
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
			"title":        t["title"],
			"notes":        t["notes"],
			"category":     t["category"],
			"order":        t["order"],
			"done":         false,
		}
		payload, _ := json.Marshal(ent)
		table.UpsertEntity(ctx, payload, nil)
	case "task-updated":
		var changes map[string]interface{}
		if err := json.Unmarshal(ev.Data, &changes); err != nil {
			return
		}
		changes["PartitionKey"] = pk
		changes["RowKey"] = rk
		payload, _ := json.Marshal(changes)
		et := azcore.ETagAny
		table.UpdateEntity(ctx, payload, &aztables.UpdateEntityOptions{IfMatch: &et, UpdateMode: aztables.UpdateModeMerge})
	case "task-completed":
		ent := map[string]any{
			"PartitionKey": pk,
			"RowKey":       rk,
			"done":         true,
		}
		payload, _ := json.Marshal(ent)
		et2 := azcore.ETagAny
		table.UpdateEntity(ctx, payload, &aztables.UpdateEntityOptions{IfMatch: &et2, UpdateMode: aztables.UpdateModeMerge})
	}
}
