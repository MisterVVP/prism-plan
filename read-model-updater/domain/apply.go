package domain

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// Storage defines methods required for updating the read model.
type Storage interface {
	GetTask(ctx context.Context, pk, rk string) (*TaskEntity, error)
	InsertTask(ctx context.Context, ent TaskEntity) error
	UpdateTask(ctx context.Context, ent TaskUpdate) error
	UpsertUser(ctx context.Context, ent UserEntity) error
	GetUserSettings(ctx context.Context, id string) (*UserSettingsEntity, error)
	UpsertUserSettings(ctx context.Context, ent UserSettingsEntity) error
	UpdateUserSettings(ctx context.Context, ent UserSettingsUpdate) error
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
		ent, err := st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent != nil {
			log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("duplicate task-created event")
			return fmt.Errorf("task %s already exists", rk)
		}
		ent = &TaskEntity{
			Entity:             Entity{PartitionKey: pk, RowKey: rk},
			Title:              eventData.Title,
			Notes:              eventData.Notes,
			Category:           eventData.Category,
			Order:              eventData.Order,
			OrderType:          EdmInt32,
			Done:               false,
			DoneType:           EdmBoolean,
			EventTimestamp:     ev.Timestamp,
			EventTimestampType: EdmInt64,
		}
		return st.InsertTask(ctx, *ent)
	case TaskUpdated:
		var eventData TaskUpdatedEventData
		if err := json.Unmarshal(ev.Data, &eventData); err != nil {
			return err
		}
		ent, err := st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			log.WithField("task", rk).Error("task-updated event for missing task")
			return fmt.Errorf("task %s not found", rk)
		}
		if ev.Timestamp <= ent.EventTimestamp {
			log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-updated event")
			return fmt.Errorf("task %s received stale update", rk)
		}
		upd := TaskUpdate{Entity: Entity{PartitionKey: pk, RowKey: rk}}
		if eventData.Title != nil {
			upd.Title = eventData.Title
		}
		if eventData.Notes != nil {
			upd.Notes = eventData.Notes
		}
		if eventData.Category != nil {
			upd.Category = eventData.Category
		}
		if eventData.Order != nil {
			upd.Order = eventData.Order
			t := EdmInt32
			upd.OrderType = &t
		}
		if eventData.Done != nil {
			upd.Done = eventData.Done
			t := EdmBoolean
			upd.DoneType = &t
		}
		upd.EventTimestamp = &ev.Timestamp
		t := EdmInt64
		upd.EventTimestampType = &t
		// Only attempt an update if there's something to change.
		if upd.Title != nil || upd.Notes != nil || upd.Category != nil || upd.Order != nil || upd.Done != nil {
			return st.UpdateTask(ctx, upd)
		}
		return fmt.Errorf("task %s update had no fields", rk)
	case TaskCompleted:
		ent, err := st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			log.WithField("task", rk).Error("task-completed event for missing task")
			return fmt.Errorf("task %s not found", rk)
		}
		if ev.Timestamp <= ent.EventTimestamp {
			log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-completed event")
			return fmt.Errorf("task %s received stale completion", rk)
		}
		done := true
		dt := EdmBoolean
		ts := ev.Timestamp
		tp := EdmInt64
		upd := TaskUpdate{
			Entity:             Entity{PartitionKey: pk, RowKey: rk},
			Done:               &done,
			DoneType:           &dt,
			EventTimestamp:     &ts,
			EventTimestampType: &tp,
		}
		return st.UpdateTask(ctx, upd)
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
	case UserLoggedIn:
		log.Infof("User logged in. UserID: %s", ev.UserID)
	case UserLoggedOut:
		log.Infof("User logged out. UserID: %s", ev.UserID)
	case UserSettingsCreated:
		var s UserSettingsEventData
		if err := json.Unmarshal(ev.Data, &s); err != nil {
			return err
		}
		ent, err := st.GetUserSettings(ctx, rk)
		if err != nil {
			return err
		}
		if ent != nil {
			log.WithFields(log.Fields{"settings": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("duplicate settings-created event")
			return fmt.Errorf("settings %s already exists", rk)
		}
		ent = &UserSettingsEntity{
			Entity:               Entity{PartitionKey: rk, RowKey: rk},
			TasksPerCategory:     s.TasksPerCategory,
			TasksPerCategoryType: EdmInt32,
			ShowDoneTasks:        s.ShowDoneTasks,
			ShowDoneTasksType:    EdmBoolean,
			EventTimestamp:       ev.Timestamp,
			EventTimestampType:   EdmInt64,
		}
		return st.UpsertUserSettings(ctx, *ent)
	case UserSettingsUpdated:
		var s UserSettingsUpdatedEventData
		if err := json.Unmarshal(ev.Data, &s); err != nil {
			return err
		}
               ent, err := st.GetUserSettings(ctx, rk)
               if err != nil {
                       return err
               }
               if ent == nil {
                       // Create settings entity if it doesn't exist yet.
                       newEnt := UserSettingsEntity{
                               Entity:               Entity{PartitionKey: rk, RowKey: rk},
                               TasksPerCategory:     0,
                               TasksPerCategoryType: EdmInt32,
                               ShowDoneTasks:        false,
                               ShowDoneTasksType:    EdmBoolean,
                               EventTimestamp:       ev.Timestamp,
                               EventTimestampType:   EdmInt64,
                       }
                       if s.TasksPerCategory != nil {
                               newEnt.TasksPerCategory = *s.TasksPerCategory
                       }
                       if s.ShowDoneTasks != nil {
                               newEnt.ShowDoneTasks = *s.ShowDoneTasks
                       }
                       return st.UpsertUserSettings(ctx, newEnt)
               }
               if ev.Timestamp <= ent.EventTimestamp {
                       log.WithFields(log.Fields{"settings": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale settings-updated event")
                       return fmt.Errorf("settings %s received stale update", rk)
               }
               upd := UserSettingsUpdate{Entity: Entity{PartitionKey: rk, RowKey: rk}}
               if s.TasksPerCategory != nil {
                       upd.TasksPerCategory = s.TasksPerCategory
                       t := EdmInt32
                       upd.TasksPerCategoryType = &t
               }
               if s.ShowDoneTasks != nil {
                       upd.ShowDoneTasks = s.ShowDoneTasks
                       t := EdmBoolean
                       upd.ShowDoneTasksType = &t
               }
               upd.EventTimestamp = &ev.Timestamp
               t := EdmInt64
               upd.EventTimestampType = &t
               // Only attempt an update if there's something to change.
               if upd.TasksPerCategory != nil || upd.ShowDoneTasks != nil {
                       return st.UpdateUserSettings(ctx, upd)
               }
               return fmt.Errorf("settings %s update had no fields", rk)
        }
        return nil
}
