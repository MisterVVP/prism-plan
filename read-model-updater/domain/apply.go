package domain

import (
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

// Storage defines methods required for updating the read model.
type Storage interface {
	GetTask(ctx context.Context, pk, rk string) (*TaskEntity, error)
	InsertTask(ctx context.Context, ent TaskEntity) error
	UpsertTask(ctx context.Context, ent TaskEntity) error
	UpdateTask(ctx context.Context, ent TaskUpdate) error
	SetTaskDone(ctx context.Context, pk, rk string) error
	UpsertUser(ctx context.Context, ent UserEntity) error
	GetUserSettings(ctx context.Context, id string) (*UserSettingsEntity, error)
	InsertUserSettings(ctx context.Context, ent UserSettingsEntity) error
	UpsertUserSettings(ctx context.Context, ent UserSettingsEntity) error
	UpdateUserSettings(ctx context.Context, ent UserSettingsUpdate) error
}

// Apply routes events to the appropriate processors.
func Apply(ctx context.Context, st Storage, ev Event) error {
	switch ev.EntityType {
	case "task":
		return (TaskProcessor{}).Handle(ctx, st, ev)
	case "user-settings":
		return (SettingsProcessor{}).Handle(ctx, st, ev)
	case "user":
		return applyUser(ctx, st, ev)
	default:
		return nil
	}
}

func applyUser(ctx context.Context, st Storage, ev Event) error {
	switch ev.Type {
	case UserCreated:
		var user UserEventData
		if err := json.Unmarshal(ev.Data, &user); err != nil {
			return err
		}
		ent := UserEntity{
			Entity: Entity{PartitionKey: ev.EntityID, RowKey: ev.EntityID},
			Name:   user.Name,
			Email:  user.Email,
		}
		return st.UpsertUser(ctx, ent)
	case UserLoggedIn:
		log.Infof("User logged in. UserID: %s", ev.UserID)
	case UserLoggedOut:
		log.Infof("User logged out. UserID: %s", ev.UserID)
	}
	return nil
}
