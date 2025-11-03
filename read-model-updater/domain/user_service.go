package domain

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// UserStorage defines methods required for updating user read models.
type UserStorage interface {
	UpsertUser(ctx context.Context, ent UserEntity) error
	GetUserSettings(ctx context.Context, id string) (*UserSettingsEntity, error)
	UpsertUserSettings(ctx context.Context, ent UserSettingsEntity) error
	UpdateUserSettings(ctx context.Context, ent UserSettingsUpdate) error
}

// UserService processes user and settings events.
type UserService struct{ st UserStorage }

func NewUserService(st UserStorage) UserService { return UserService{st: st} }

// Apply updates the read model for user related events.
func (s UserService) Apply(ctx context.Context, ev Event) error {
	rk := ev.EntityID
	switch ev.Type {
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
		return s.st.UpsertUser(ctx, ent)
	case UserLoggedIn:
		log.Infof("User logged in. UserID: %s", ev.UserID)
	case UserLoggedOut:
		log.Infof("User logged out. UserID: %s", ev.UserID)
	case UserSettingsCreated:
		var sEvent UserSettingsEventData
		if err := json.Unmarshal(ev.Data, &sEvent); err != nil {
			return err
		}
		ent, err := s.st.GetUserSettings(ctx, rk)
		if err != nil {
			return err
		}
		if ent != nil {
			log.WithFields(log.Fields{"settings": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("duplicate settings-created event")
			return fmt.Errorf("settings %s already exists", rk)
		}
		ent = &UserSettingsEntity{
			Entity:           Entity{PartitionKey: rk, RowKey: rk},
			TasksPerCategory: sEvent.TasksPerCategory,
			ShowDoneTasks:    sEvent.ShowDoneTasks,
			EventTimestamp:   ev.Timestamp,
		}
		return s.st.UpsertUserSettings(ctx, *ent)
	case UserSettingsUpdated:
		var sUpd UserSettingsUpdatedEventData
		if err := json.Unmarshal(ev.Data, &sUpd); err != nil {
			return err
		}
		ent, err := s.st.GetUserSettings(ctx, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			newEnt := UserSettingsEntity{
				Entity:           Entity{PartitionKey: rk, RowKey: rk},
				TasksPerCategory: 0,
				ShowDoneTasks:    false,
				EventTimestamp:   ev.Timestamp,
			}
			if sUpd.TasksPerCategory != nil {
				newEnt.TasksPerCategory = *sUpd.TasksPerCategory
			}
			if sUpd.ShowDoneTasks != nil {
				newEnt.ShowDoneTasks = *sUpd.ShowDoneTasks
			}
			return s.st.UpsertUserSettings(ctx, newEnt)
		}
		if ev.Timestamp <= ent.EventTimestamp {
			log.WithFields(log.Fields{"settings": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale settings-updated event")
			return fmt.Errorf("settings %s received stale update", rk)
		}
		upd := UserSettingsUpdate{Entity: Entity{PartitionKey: rk, RowKey: rk}}
		if sUpd.TasksPerCategory != nil {
			upd.TasksPerCategory = sUpd.TasksPerCategory
		}
		if sUpd.ShowDoneTasks != nil {
			upd.ShowDoneTasks = sUpd.ShowDoneTasks
		}
		upd.EventTimestamp = &ev.Timestamp
		if upd.TasksPerCategory != nil || upd.ShowDoneTasks != nil {
			return s.st.UpdateUserSettings(ctx, upd)
		}
		return fmt.Errorf("settings %s update had no fields", rk)
	default:
		return fmt.Errorf("unknown user event %s", ev.Type)
	}
	return nil
}
