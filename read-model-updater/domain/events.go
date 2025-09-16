package domain

import "encoding/json"

const (
	TaskCreated         = "task-created"
	TaskUpdated         = "task-updated"
	TaskCompleted       = "task-completed"
	TaskReopened        = "task-reopened"
	UserCreated         = "user-created"
	UserLoggedIn        = "user-logged-in"
	UserLoggedOut       = "user-logged-out"
	UserSettingsCreated = "user-settings-created"
	UserSettingsUpdated = "user-settings-updated"
)

// Event represents a change in the domain model.
type Event struct {
	ID         string          `json:"Id"`
	EntityID   string          `json:"EntityId"`
	EntityType string          `json:"EntityType"`
	Type       string          `json:"Type"`
	Data       json.RawMessage `json:"Data"`
	Timestamp  int64           `json:"Timestamp"`
	UserID     string          `json:"UserId"`
}

type UserEventData struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

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
	Done     *bool   `json:"done"`
}

type UserSettingsEventData struct {
	TasksPerCategory int  `json:"tasksPerCategory"`
	ShowDoneTasks    bool `json:"showDoneTasks"`
}

type UserSettingsUpdatedEventData struct {
	TasksPerCategory *int  `json:"tasksPerCategory"`
	ShowDoneTasks    *bool `json:"showDoneTasks"`
}
