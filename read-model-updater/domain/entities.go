package domain

// Entity represents base table entity keys.
type Entity struct {
	PartitionKey string `json:"PartitionKey"`
	RowKey       string `json:"RowKey"`
}

const (
	EdmInt32   = "Edm.Int32"
	EdmBoolean = "Edm.Boolean"
)

// TaskEntity represents a task stored in the read model.
type TaskEntity struct {
	Entity
	Title     string `json:"Title,omitempty"`
	Notes     string `json:"Notes,omitempty"`
	Category  string `json:"Category,omitempty"`
	Order     int    `json:"Order"`
	OrderType string `json:"Order@odata.type"`
	Done      bool   `json:"Done"`
	DoneType  string `json:"Done@odata.type"`
}

// TaskUpdate carries partial updates for a task.
type TaskUpdate struct {
	Entity
	Title     *string `json:"Title,omitempty"`
	Notes     *string `json:"Notes,omitempty"`
	Category  *string `json:"Category,omitempty"`
	Order     *int    `json:"Order,omitempty"`
	OrderType *string `json:"Order@odata.type,omitempty"`
	Done      *bool   `json:"Done,omitempty"`
	DoneType  *string `json:"Done@odata.type,omitempty"`
}

// UserEntity represents a user stored in the read model.
type UserEntity struct {
	Entity
	Name  string `json:"Name,omitempty"`
	Email string `json:"Email,omitempty"`
}
