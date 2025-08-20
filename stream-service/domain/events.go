package domain

import "encoding/json"

const (
	TaskCreated   = "task-created"
	TaskUpdated   = "task-updated"
	TaskCompleted = "task-completed"
	UserCreated   = "user-created"
	UserLoggedIn  = "user-logged-in"
	UserLoggedOut = "user-logged-out"
)

type TaskCreatedEventData struct {
	Title    string `json:"title"`
	Notes    string `json:"notes"`
	Category string `json:"category"`
	Order    int    `json:"order"`
}

type TaskUpdatedEventData struct {
	Title    *string `json:"title"`
	Notes    *string `json:"notes"`
	Category *string `json:"category"`
	Order    *int    `json:"order"`
}

type TaskEvent struct {
	EntityID string          `json:"EntityId"`
	Type     string          `json:"Type"`
	Data     json.RawMessage `json:"Data"`
	UserID   string          `json:"UserId"`
}
