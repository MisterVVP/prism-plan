package api

import (
	"context"
	"prism-api/domain"
)

// Storage abstracts persistence for handlers.
type Storage interface {
	FetchTasks(ctx context.Context, userID string) ([]domain.Task, error)
	FetchSettings(ctx context.Context, userID string) (domain.Settings, error)
	EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error
}

// Authenticator is implemented by types able to extract user IDs from headers.
type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}

// Deduper prevents processing of duplicate commands.
type Deduper interface {
	// Add records the idempotency key and returns true if it was newly added.
	Add(ctx context.Context, userID, key string) (bool, error)
	// Remove deletes a previously added key, used when downstream processing fails.
	Remove(ctx context.Context, userID, key string) error
}
