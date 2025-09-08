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
	Title              string `json:"Title,omitempty"`
	Notes              string `json:"Notes,omitempty"`
	Category           string `json:"Category,omitempty"`
	Order              *int   `json:"Order,omitempty"`
	Done               *bool  `json:"Done,omitempty"`
	EventTimestamp     int64  `json:"EventTimestamp,string"`
	EventTimestampType string `json:"EventTimestamp@odata.type"`
}

// TaskUpdate carries partial updates for a task.
type TaskUpdate struct {
	Entity
	Title              *string `json:"Title,omitempty"`
	Notes              *string `json:"Notes,omitempty"`
	Category           *string `json:"Category,omitempty"`
	Order              *int    `json:"Order,omitempty"`
	Done               *bool   `json:"Done,omitempty"`
	EventTimestamp     *int64  `json:"EventTimestamp,omitempty,string"`
	EventTimestampType *string `json:"EventTimestamp@odata.type,omitempty"`
}

// UserEntity represents a user stored in the read model.
type UserEntity struct {
	Entity
	Name  string `json:"Name,omitempty"`
	Email string `json:"Email,omitempty"`
}

type UserSettingsEntity struct {
	Entity
	TasksPerCategory     int    `json:"TasksPerCategory"`
	TasksPerCategoryType string `json:"TasksPerCategory@odata.type"`
	ShowDoneTasks        bool   `json:"ShowDoneTasks"`
	ShowDoneTasksType    string `json:"ShowDoneTasks@odata.type"`
	EventTimestamp       int64  `json:"EventTimestamp,string"`
	EventTimestampType   string `json:"EventTimestamp@odata.type"`
}

type UserSettingsUpdate struct {
	Entity
	TasksPerCategory     *int    `json:"TasksPerCategory,omitempty"`
	TasksPerCategoryType *string `json:"TasksPerCategory@odata.type,omitempty"`
	ShowDoneTasks        *bool   `json:"ShowDoneTasks,omitempty"`
	ShowDoneTasksType    *string `json:"ShowDoneTasks@odata.type,omitempty"`
	EventTimestamp       *int64  `json:"EventTimestamp,omitempty,string"`
	EventTimestampType   *string `json:"EventTimestamp@odata.type,omitempty"`
}
