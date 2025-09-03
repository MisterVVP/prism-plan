package domain

import (
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

// Storage defines methods required for updating the read model.
type Storage interface {
	GetTask(ctx context.Context, pk, rk string) (*TaskEntity, error)
	UpsertTask(ctx context.Context, ent TaskEntity) error
	UpdateTask(ctx context.Context, ent TaskUpdate) error
	SetTaskDone(ctx context.Context, pk, rk string) error
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
		if ent == nil {
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
			return st.UpsertTask(ctx, *ent)
		}
		if ev.Timestamp == ent.EventTimestamp {
			log.Warnf("task %s received event with identical timestamp", rk)
		}
		if ev.Timestamp >= ent.EventTimestamp {
			ent.Title = eventData.Title
			ent.Notes = eventData.Notes
			ent.Category = eventData.Category
			ent.Order = eventData.Order
			ent.OrderType = EdmInt32
			ent.Done = false
			ent.DoneType = EdmBoolean
			ent.EventTimestamp = ev.Timestamp
			ent.EventTimestampType = EdmInt64
		} else {
			if ent.Title == "" {
				ent.Title = eventData.Title
			}
			if ent.Notes == "" {
				ent.Notes = eventData.Notes
			}
			if ent.Category == "" {
				ent.Category = eventData.Category
			}
			if ent.Order == 0 {
				ent.Order = eventData.Order
				ent.OrderType = EdmInt32
			}
		}
		return st.UpsertTask(ctx, *ent)
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
			ent = &TaskEntity{Entity: Entity{PartitionKey: pk, RowKey: rk}, OrderType: EdmInt32, DoneType: EdmBoolean, EventTimestamp: ev.Timestamp, EventTimestampType: EdmInt64}
			if eventData.Title != nil {
				ent.Title = *eventData.Title
			}
			if eventData.Notes != nil {
				ent.Notes = *eventData.Notes
			}
			if eventData.Category != nil {
				ent.Category = *eventData.Category
			}
			if eventData.Order != nil {
				ent.Order = *eventData.Order
			}
			if eventData.Done != nil {
				ent.Done = *eventData.Done
			}
			return st.UpsertTask(ctx, *ent)
		}
		if ev.Timestamp == ent.EventTimestamp {
			log.Warnf("task %s received event with identical timestamp", rk)
		}
		if ev.Timestamp < ent.EventTimestamp {
			return nil
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
		if upd.Title != nil || upd.Notes != nil || upd.Category != nil || upd.Order != nil || upd.Done != nil || upd.EventTimestamp != nil {
			return st.UpdateTask(ctx, upd)
		}
		return nil
	case TaskCompleted:
		ent, err := st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			ent = &TaskEntity{Entity: Entity{PartitionKey: pk, RowKey: rk}, Done: true, DoneType: EdmBoolean, EventTimestamp: ev.Timestamp, EventTimestampType: EdmInt64}
			return st.UpsertTask(ctx, *ent)
		}
		if ev.Timestamp == ent.EventTimestamp {
			log.Warnf("task %s received event with identical timestamp", rk)
		}
		if ev.Timestamp >= ent.EventTimestamp {
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
		}
		return nil
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
		if ent == nil {
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
		}
		if ev.Timestamp == ent.EventTimestamp {
			log.Warnf("settings %s received event with identical timestamp", rk)
		}
		if ev.Timestamp >= ent.EventTimestamp {
			ent.TasksPerCategory = s.TasksPerCategory
			ent.TasksPerCategoryType = EdmInt32
			ent.ShowDoneTasks = s.ShowDoneTasks
			ent.ShowDoneTasksType = EdmBoolean
			ent.EventTimestamp = ev.Timestamp
			ent.EventTimestampType = EdmInt64
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
			ent = &UserSettingsEntity{Entity: Entity{PartitionKey: rk, RowKey: rk}, EventTimestamp: ev.Timestamp, EventTimestampType: EdmInt64}
			if s.TasksPerCategory != nil {
				ent.TasksPerCategory = *s.TasksPerCategory
				ent.TasksPerCategoryType = EdmInt32
			}
			if s.ShowDoneTasks != nil {
				ent.ShowDoneTasks = *s.ShowDoneTasks
				ent.ShowDoneTasksType = EdmBoolean
			}
			return st.UpsertUserSettings(ctx, *ent)
		}
		if ev.Timestamp == ent.EventTimestamp {
			log.Warnf("settings %s received event with identical timestamp", rk)
		}
		if ev.Timestamp >= ent.EventTimestamp {
			if s.TasksPerCategory != nil {
				ent.TasksPerCategory = *s.TasksPerCategory
				ent.TasksPerCategoryType = EdmInt32
			}
			if s.ShowDoneTasks != nil {
				ent.ShowDoneTasks = *s.ShowDoneTasks
				ent.ShowDoneTasksType = EdmBoolean
			}
			ent.EventTimestamp = ev.Timestamp
			ent.EventTimestampType = EdmInt64
		}
		return st.UpsertUserSettings(ctx, *ent)
	}
	return nil
}
