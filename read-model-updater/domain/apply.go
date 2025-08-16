package domain

import (
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

// Storage defines methods required for updating the read model.
type Storage interface {
	UpsertTask(ctx context.Context, ent TaskEntity) error
	UpdateTask(ctx context.Context, ent TaskUpdate) error
	SetTaskDone(ctx context.Context, pk, rk string) error
	UpsertUser(ctx context.Context, ent UserEntity) error
}

// Apply updates the read model based on an incoming event.
func Apply(ctx context.Context, st Storage, ev Event) error {
	pk := ev.UserID
	rk := ev.EntityID
	switch ev.Type {
	case TaskCreated:
		var eventData TaskCreatedEventData
		if err := json.Unmarshal(ev.Data, &eventData); err != nil {
			return err
		}
		ent := TaskEntity{
			Entity:    Entity{PartitionKey: pk, RowKey: rk},
			Title:     eventData.Title,
			Notes:     eventData.Notes,
			Category:  eventData.Category,
			Order:     int(eventData.Order),
			OrderType: EdmInt32,
			Done:      false,
			DoneType:  EdmBoolean,
		}
		return st.UpsertTask(ctx, ent)
	case TaskUpdated:
		var eventData TaskUpdatedEventData
		if err := json.Unmarshal(ev.Data, &eventData); err != nil {
			return err
		}
		updates := TaskUpdate{Entity: Entity{PartitionKey: pk, RowKey: rk}}
		if eventData.Title != nil {
			updates.Title = eventData.Title
		}
		if eventData.Notes != nil {
			updates.Notes = eventData.Notes
		}
		if eventData.Category != nil {
			updates.Category = eventData.Category
		}
		if eventData.Order != nil {
			v := int(*eventData.Order)
			updates.Order = &v
			t := EdmInt32
			updates.OrderType = &t
		}
		return st.UpdateTask(ctx, updates)
	case TaskCompleted:
		return st.SetTaskDone(ctx, pk, rk)
	case UserCreated:
		var user UserEventData
		if err := json.Unmarshal(ev.Data, &user); err != nil {
			return err
		}
		ent := UserEntity{
			Entity: Entity{PartitionKey: rk, RowKey: rk},
			Name:   user.Name,
			Email:  user.Email,
		}
		return st.UpsertUser(ctx, ent)
	case UserLoggedIn, UserLoggedOut:
		var user UserEventData
		if err := json.Unmarshal(ev.Data, &user); err != nil {
			return err
		}
		log.Infof("User logged in, name: %s email: %s", user.Name, user.Email)
	}
	return nil
}
