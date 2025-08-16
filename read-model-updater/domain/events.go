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

// Event represents a change in the domain model.
type Event struct {
	ID         string          `json:"Id"`
	EntityID   string          `json:"EntityId"`
	EntityType string          `json:"EntityType"`
	Type       string          `json:"Type"`
	Data       json.RawMessage `json:"Data"`
	Time       int64           `json:"Time"`
	UserID     string          `json:"UserId"`
}

type UserEventData struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type TaskCreatedEventData struct {
	Title    string  `json:"title"`
	Notes    string  `json:"notes"`
	Category string  `json:"category"`
	Order    float64 `json:"order"`
}

type TaskUpdatedEventData struct {
	Title    *string  `json:"title"`
	Notes    *string  `json:"notes"`
	Category *string  `json:"category"`
	Order    *float64 `json:"order"`
}
