package api

import (
	"context"
	"prism-api/domain"
)

// Storage abstracts persistence for handlers.
type Storage interface {
	FetchTasks(ctx context.Context, userID, continuationToken string, limit int) ([]domain.Task, string, error)
	FetchSettings(ctx context.Context, userID string) (domain.Settings, error)
	EnqueueCommands(ctx context.Context, userID string, cmds []domain.Command) error
}

// InvalidContinuationTokenError is returned when a supplied pagination token is malformed or expired.
type InvalidContinuationTokenError interface {
	error
	InvalidContinuationToken()
}

// Authenticator is implemented by types able to extract user IDs from headers.
type Authenticator interface {
	UserIDFromAuthHeader(string) (string, error)
}
