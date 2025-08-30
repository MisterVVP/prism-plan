package domain

import "encoding/json"

// Command represents a write request for the domain model.
type Command struct {
	ID         string          `json:"id"`
	EntityID   string          `json:"entityId"`
	EntityType string          `json:"entityType"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data,omitempty"`
	Timestamp  int64           `json:"timestamp"`
}

// CommandEnvelope wraps a command with the user performing it.
type CommandEnvelope struct {
	UserID  string  `json:"userId"`
	Command Command `json:"command"`
}
