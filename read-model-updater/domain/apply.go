package domain

import (
	"context"
	"encoding/json"
)

// Storage defines methods required for updating the read model.
type Storage interface {
	UpsertTask(ctx context.Context, ent TaskEntity) error
	UpdateTask(ctx context.Context, ent TaskUpdate) error
	SetTaskDone(ctx context.Context, pk, rk string) error
	UpsertUser(ctx context.Context, ent UserEntity) error
}

// Apply updates the read model based on an incoming event.
func Apply(ctx context.Context, st Storage, ev Event) {
	pk := ev.UserID
	rk := ev.EntityID
	switch ev.Type {
	case TaskCreated:
		var t struct {
			Title    string `json:"title"`
			Notes    string `json:"notes"`
			Category string `json:"category"`
			Order    int    `json:"order"`
		}
		if err := json.Unmarshal(ev.Data, &t); err != nil {
			return
		}
		ent := TaskEntity{
			Entity:   Entity{PartitionKey: pk, RowKey: rk},
			Title:    t.Title,
			Notes:    t.Notes,
			Category: t.Category,
			Order:    t.Order,
			Done:     false,
		}
		st.UpsertTask(ctx, ent)
	case TaskUpdated:
		var changes struct {
			Title    *string `json:"title"`
			Notes    *string `json:"notes"`
			Category *string `json:"category"`
			Order    *int    `json:"order"`
		}
		if err := json.Unmarshal(ev.Data, &changes); err != nil {
			return
		}
		updates := TaskUpdate{Entity: Entity{PartitionKey: pk, RowKey: rk}}
		if changes.Title != nil {
			updates.Title = changes.Title
		}
		if changes.Notes != nil {
			updates.Notes = changes.Notes
		}
		if changes.Category != nil {
			updates.Category = changes.Category
		}
		if changes.Order != nil {
			updates.Order = changes.Order
		}
		st.UpdateTask(ctx, updates)
	case TaskCompleted:
		st.SetTaskDone(ctx, pk, rk)
	case UserCreated:
		var u struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := json.Unmarshal(ev.Data, &u); err != nil {
			return
		}
		ent := UserEntity{
			Entity: Entity{PartitionKey: rk, RowKey: rk},
			Name:   u.Name,
			Email:  u.Email,
		}
		st.UpsertUser(ctx, ent)
	case UserLoggedIn, UserLoggedOut:
		// no-op
	}
}
