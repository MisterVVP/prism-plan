package domain

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// SettingsProcessor handles user settings events.
type SettingsProcessor struct{}

type settingsFields struct {
	TasksPerCategory *int
	ShowDoneTasks    *bool
}

func ensureSettings(ctx context.Context, st Storage, id string, ts int64, f settingsFields) (*UserSettingsEntity, bool, error) {
	ent, err := st.GetUserSettings(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if ent != nil {
		return ent, false, nil
	}
	ent = &UserSettingsEntity{Entity: Entity{PartitionKey: id, RowKey: id}, TasksPerCategoryType: EdmInt32, ShowDoneTasksType: EdmBoolean, EventTimestamp: ts, EventTimestampType: EdmInt64}
	if f.TasksPerCategory != nil {
		ent.TasksPerCategory = *f.TasksPerCategory
	}
	if f.ShowDoneTasks != nil {
		ent.ShowDoneTasks = *f.ShowDoneTasks
	}
	if err := st.InsertUserSettings(ctx, *ent); err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			ent, err = st.GetUserSettings(ctx, id)
			if err != nil {
				return nil, false, err
			}
			return ent, false, nil
		}
		return nil, false, err
	}
	return ent, true, nil
}

func mergeSettings(ent *UserSettingsEntity, ts int64, f settingsFields) (UserSettingsUpdate, bool, error) {
	if ts == ent.EventTimestamp {
		// duplicate event, no changes required
		return UserSettingsUpdate{}, false, nil
	}
	upd := UserSettingsUpdate{Entity: ent.Entity}
	changed := false
	if ts > ent.EventTimestamp {
		if f.TasksPerCategory != nil {
			upd.TasksPerCategory = f.TasksPerCategory
			t := EdmInt32
			upd.TasksPerCategoryType = &t
			ent.TasksPerCategory = *f.TasksPerCategory
			ent.TasksPerCategoryType = EdmInt32
		}
		if f.ShowDoneTasks != nil {
			upd.ShowDoneTasks = f.ShowDoneTasks
			t := EdmBoolean
			upd.ShowDoneTasksType = &t
			ent.ShowDoneTasks = *f.ShowDoneTasks
			ent.ShowDoneTasksType = EdmBoolean
		}
		upd.EventTimestamp = &ts
		t := EdmInt64
		upd.EventTimestampType = &t
		ent.EventTimestamp = ts
		ent.EventTimestampType = EdmInt64
		changed = upd.TasksPerCategory != nil || upd.ShowDoneTasks != nil
	} else {
		if f.TasksPerCategory != nil && ent.TasksPerCategory == 0 {
			upd.TasksPerCategory = f.TasksPerCategory
			t := EdmInt32
			upd.TasksPerCategoryType = &t
			ent.TasksPerCategory = *f.TasksPerCategory
			ent.TasksPerCategoryType = EdmInt32
			changed = true
		}
		if f.ShowDoneTasks != nil && ent.ShowDoneTasksType == "" {
			upd.ShowDoneTasks = f.ShowDoneTasks
			t := EdmBoolean
			upd.ShowDoneTasksType = &t
			ent.ShowDoneTasks = *f.ShowDoneTasks
			ent.ShowDoneTasksType = EdmBoolean
			changed = true
		}
	}
	if upd.EventTimestamp != nil || changed {
		return upd, true, nil
	}
	return UserSettingsUpdate{}, false, nil
}

// Handle processes user settings events.
func (SettingsProcessor) Handle(ctx context.Context, st Storage, ev Event) error {
	id := ev.EntityID
	switch ev.Type {
	case UserSettingsCreated:
		var data UserSettingsEventData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return err
		}
		f := settingsFields{TasksPerCategory: &data.TasksPerCategory, ShowDoneTasks: &data.ShowDoneTasks}
		ent, created, err := ensureSettings(ctx, st, id, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if created {
			return nil
		}
		upd, changed, err := mergeSettings(ent, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if changed {
			return st.UpdateUserSettings(ctx, upd)
		}
		return nil
	case UserSettingsUpdated:
		var data UserSettingsUpdatedEventData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return err
		}
		f := settingsFields{TasksPerCategory: data.TasksPerCategory, ShowDoneTasks: data.ShowDoneTasks}
		ent, created, err := ensureSettings(ctx, st, id, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if created {
			return nil
		}
		upd, changed, err := mergeSettings(ent, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if changed {
			return st.UpdateUserSettings(ctx, upd)
		}
		return nil
	}
	return nil
}
