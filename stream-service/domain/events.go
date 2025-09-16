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
	TasksPerCategory *int  `json:"tasksPerCategory"`
	ShowDoneTasks    *bool `json:"showDoneTasks"`
}

type Event struct {
	EntityID   string          `json:"EntityId"`
	EntityType string          `json:"EntityType"`
	Type       string          `json:"Type"`
	Data       json.RawMessage `json:"Data"`
	UserID     string          `json:"UserId"`
}
