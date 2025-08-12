package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"read-model-updater/domain"
	"read-model-updater/storage"
)

func main() {
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
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Data struct {
				QueueTrigger string `json:"queueTrigger"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var ev domain.Event
		if err := json.Unmarshal([]byte(payload.Data.QueueTrigger), &ev); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		domain.Apply(r.Context(), st, ev)
		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
