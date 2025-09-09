package domain

import (
	"context"
	"fmt"
)

// Orchestrator routes events to the appropriate service based on entity type.
type Orchestrator struct {
	tasks TaskService
	users UserService
}

func NewOrchestrator(tasks TaskService, users UserService) Orchestrator {
	return Orchestrator{tasks: tasks, users: users}
}

// Apply delegates event handling to the corresponding service.
func (o Orchestrator) Apply(ctx context.Context, ev Event) error {
	switch ev.EntityType {
	case "task":
		return o.tasks.Apply(ctx, ev)
	case "user", "user-settings":
		return o.users.Apply(ctx, ev)
	default:
		return fmt.Errorf("unknown entity type %s", ev.EntityType)
	}
}
