package domain

import "encoding/json"

// Event represents a change in the domain model.
type Event struct {
	ID         string          `json:"id"`
	EntityID   string          `json:"entityId"`
	EntityType string          `json:"entityType"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	Time       int64           `json:"time"`
	UserID     string          `json:"userId"`
}
