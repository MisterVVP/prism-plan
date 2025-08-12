package domain

import (
	"context"
	"encoding/json"
	"strings"
)

// Storage defines methods required for updating the read model.
type Storage interface {
	UpsertTask(ctx context.Context, ent map[string]any) error
	UpdateTask(ctx context.Context, ent map[string]any) error
	SetTaskDone(ctx context.Context, pk, rk string) error
	UpsertUser(ctx context.Context, ent map[string]any) error
}

// Apply updates the read model based on an incoming event.
func Apply(ctx context.Context, st Storage, ev Event) {
	pk := ev.UserID
	rk := ev.EntityID
	switch ev.Type {
	case "task-created":
		var t map[string]any
		if err := json.Unmarshal(ev.Data, &t); err != nil {
			return
		}
		ent := map[string]any{
			"PartitionKey": pk,
			"RowKey":       rk,
			"Title":        t["title"],
			"Notes":        t["notes"],
			"Category":     t["category"],
			"Order":        t["order"],
			"Done":         false,
		}
		st.UpsertTask(ctx, ent)
	case "task-updated":
		var changes map[string]any
		if err := json.Unmarshal(ev.Data, &changes); err != nil {
			return
		}
		updates := map[string]any{
			"PartitionKey": pk,
			"RowKey":       rk,
		}
		for k, v := range changes {
			if k == "" {
				continue
			}
			capKey := strings.ToUpper(k[:1]) + k[1:]
			updates[capKey] = v
		}
		st.UpdateTask(ctx, updates)
	case "task-completed":
		st.SetTaskDone(ctx, pk, rk)
	case "user-created":
		var u map[string]any
		if err := json.Unmarshal(ev.Data, &u); err != nil {
			return
		}
		ent := map[string]any{
			"PartitionKey": rk,
			"RowKey":       rk,
			"Name":         u["name"],
			"Email":        u["email"],
		}
		st.UpsertUser(ctx, ent)
	case "user-logged-in", "user-logged-out":
		// no-op
	}
}
