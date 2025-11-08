package domain

// Entity represents base table entity keys.
type Entity struct {
	PartitionKey string `json:"PartitionKey"`
	RowKey       string `json:"RowKey"`
}

const (
	EdmInt32   = "Edm.Int32"
	EdmBoolean = "Edm.Boolean"
	EdmInt64   = "Edm.Int64"
)

// TaskEntity represents a task stored in the read model.
type TaskEntity struct {
	Entity
	Title          string `json:"Title,omitempty"`
	Notes          string `json:"Notes,omitempty"`
	Category       string `json:"Category,omitempty"`
	Order          int    `json:"Order"`
	Done           bool   `json:"Done"`
	EventTimestamp int64  `json:"EventTimestamp,string"`
	ETag           string `json:"-"`
}

// TaskUpdate carries partial updates for a task.
type TaskUpdate struct {
	Entity
	Title          *string `json:"Title,omitempty"`
	Notes          *string `json:"Notes,omitempty"`
	Category       *string `json:"Category,omitempty"`
	Order          *int    `json:"Order,omitempty"`
	Done           *bool   `json:"Done,omitempty"`
	EventTimestamp *int64  `json:"EventTimestamp,omitempty,string"`
}

// UserEntity represents a user stored in the read model.
type UserEntity struct {
	Entity
	Name  string `json:"Name,omitempty"`
	Email string `json:"Email,omitempty"`
}

type UserSettingsEntity struct {
	Entity
	TasksPerCategory int   `json:"TasksPerCategory"`
	ShowDoneTasks    bool  `json:"ShowDoneTasks"`
	EventTimestamp   int64 `json:"EventTimestamp,string"`
}

type UserSettingsUpdate struct {
	Entity
	TasksPerCategory *int   `json:"TasksPerCategory,omitempty"`
	ShowDoneTasks    *bool  `json:"ShowDoneTasks,omitempty"`
	EventTimestamp   *int64 `json:"EventTimestamp,omitempty,string"`
}
