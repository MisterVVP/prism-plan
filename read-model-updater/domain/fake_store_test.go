package domain

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

type fakeStore struct {
	tasks          map[string]TaskEntity
	settings       map[string]UserSettingsEntity
	upsertTask     TaskEntity
	insertTask     TaskEntity
	updateTask     TaskUpdate
	upsertUser     UserEntity
	upsertSettings UserSettingsEntity
	updateSettings UserSettingsUpdate
}

func (f *fakeStore) GetTask(ctx context.Context, pk, rk string) (*TaskEntity, error) {
	if f.tasks == nil {
		return nil, nil
	}
	ent, ok := f.tasks[rk]
	if !ok {
		return nil, nil
	}
	return &ent, nil
}

func (f *fakeStore) InsertTask(ctx context.Context, ent TaskEntity) error {
	if f.tasks == nil {
		f.tasks = map[string]TaskEntity{}
	}
	if _, exists := f.tasks[ent.RowKey]; exists {
		return &azcore.ResponseError{StatusCode: 409}
	}
	f.tasks[ent.RowKey] = ent
	f.insertTask = ent
	return nil
}

func (f *fakeStore) UpsertTask(ctx context.Context, ent TaskEntity) error {
	if f.tasks == nil {
		f.tasks = map[string]TaskEntity{}
	}
	f.tasks[ent.RowKey] = ent
	f.upsertTask = ent
	return nil
}

func (f *fakeStore) UpdateTask(ctx context.Context, upd TaskUpdate) error {
	if f.tasks == nil {
		f.tasks = map[string]TaskEntity{}
	}
	ent, ok := f.tasks[upd.RowKey]
	if !ok {
		ent = TaskEntity{Entity: Entity{PartitionKey: upd.PartitionKey, RowKey: upd.RowKey}}
	}
	if upd.Title != nil {
		ent.Title = *upd.Title
	}
	if upd.Notes != nil {
		ent.Notes = *upd.Notes
	}
	if upd.Category != nil {
		ent.Category = *upd.Category
	}
	if upd.Order != nil {
		ent.Order = upd.Order
	}
	if upd.Done != nil {
		ent.Done = upd.Done
	}
	if upd.EventTimestamp != nil {
		ent.EventTimestamp = *upd.EventTimestamp
		if upd.EventTimestampType != nil {
			ent.EventTimestampType = *upd.EventTimestampType
		}
	}
	f.tasks[upd.RowKey] = ent
	f.updateTask = upd
	return nil
}

func (f *fakeStore) SetTaskDone(ctx context.Context, pk, rk string) error { return nil }

func (f *fakeStore) UpsertUser(ctx context.Context, ent UserEntity) error {
	f.upsertUser = ent
	return nil
}

func (f *fakeStore) GetUserSettings(ctx context.Context, id string) (*UserSettingsEntity, error) {
	if f.settings == nil {
		return nil, nil
	}
	ent, ok := f.settings[id]
	if !ok {
		return nil, nil
	}
	return &ent, nil
}

func (f *fakeStore) InsertUserSettings(ctx context.Context, ent UserSettingsEntity) error {
	if f.settings == nil {
		f.settings = map[string]UserSettingsEntity{}
	}
	if _, exists := f.settings[ent.RowKey]; exists {
		return &azcore.ResponseError{StatusCode: 409}
	}
	f.settings[ent.RowKey] = ent
	return nil
}

func (f *fakeStore) UpsertUserSettings(ctx context.Context, ent UserSettingsEntity) error {
	if f.settings == nil {
		f.settings = map[string]UserSettingsEntity{}
	}
	f.settings[ent.RowKey] = ent
	f.upsertSettings = ent
	return nil
}

func (f *fakeStore) UpdateUserSettings(ctx context.Context, ent UserSettingsUpdate) error {
	if f.settings == nil {
		f.settings = map[string]UserSettingsEntity{}
	}
	cur, ok := f.settings[ent.RowKey]
	if !ok {
		cur = UserSettingsEntity{Entity: Entity{PartitionKey: ent.PartitionKey, RowKey: ent.RowKey}}
	}
	if ent.TasksPerCategory != nil {
		cur.TasksPerCategory = *ent.TasksPerCategory
		if ent.TasksPerCategoryType != nil {
			cur.TasksPerCategoryType = *ent.TasksPerCategoryType
		}
	}
	if ent.ShowDoneTasks != nil {
		cur.ShowDoneTasks = *ent.ShowDoneTasks
		if ent.ShowDoneTasksType != nil {
			cur.ShowDoneTasksType = *ent.ShowDoneTasksType
		}
	}
	if ent.EventTimestamp != nil {
		cur.EventTimestamp = *ent.EventTimestamp
		if ent.EventTimestampType != nil {
			cur.EventTimestampType = *ent.EventTimestampType
		}
	}
	f.settings[ent.RowKey] = cur
	f.updateSettings = ent
	return nil
}
