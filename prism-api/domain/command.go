package domain

import "github.com/bytedance/sonic"

// Command represents a write request for the domain model.
type Command struct {
	// Id carries the idempotency key when enqueued to the domain service queue.
	ID             string                 `json:"id,omitempty"`
	IdempotencyKey string                 `json:"idempotencyKey"`
	EntityType     string                 `json:"entityType"`
	Type           string                 `json:"type"`
	Data           sonic.NoCopyRawMessage `json:"data,omitempty"`
	Timestamp      int64                  `json:"timestamp"`
}

// CommandEnvelope wraps a command with the user performing it.
type CommandEnvelope struct {
	UserID  string  `json:"userId"`
	Command Command `json:"command"`
}
