package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"read-model-updater/domain"
	"read-model-updater/storage"
)

func main() {
	log.Println("Read-Model Updater Service starting")

	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	eventsQueue := os.Getenv("DOMAIN_EVENTS_QUEUE")
	tasksTable := os.Getenv("TASKS_TABLE")
	usersTable := os.Getenv("USERS_TABLE")
	if connStr == "" || eventsQueue == "" || tasksTable == "" || usersTable == "" {
		log.Fatal("missing storage config")
	}

	st, err := storage.New(connStr, eventsQueue, tasksTable, usersTable)
	if err != nil {
		log.Fatalf("storage: %v", err)
		return
	}

	ctx := context.Background()
	for {
		msg, err := st.Dequeue(ctx)
		if err != nil {
			log.Printf("receive: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if msg == nil {
			time.Sleep(time.Second)
			continue
		}
		var ev domain.Event
		if err := json.Unmarshal([]byte(*msg.MessageText), &ev); err == nil {
			domain.Apply(ctx, st, ev)
		}
		st.Delete(ctx, *msg.MessageID, *msg.PopReceipt)
	}
}
