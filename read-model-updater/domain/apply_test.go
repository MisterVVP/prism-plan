package domain

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeStore struct {
	tasks          map[string]TaskEntity
	settings       map[string]UserSettingsEntity
	upsertTask     TaskEntity
	upsertUser     UserEntity
	upsertSettings UserSettingsEntity
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

func (f *fakeStore) UpsertTask(ctx context.Context, ent TaskEntity) error {
	if f.tasks == nil {
		f.tasks = map[string]TaskEntity{}
	}
	f.tasks[ent.RowKey] = ent
	f.upsertTask = ent
	return nil
}

func (f *fakeStore) UpdateTask(ctx context.Context, ent TaskUpdate) error { return nil }

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

func (f *fakeStore) UpsertUserSettings(ctx context.Context, ent UserSettingsEntity) error {
	if f.settings == nil {
		f.settings = map[string]UserSettingsEntity{}
	}
	f.settings[ent.RowKey] = ent
	f.upsertSettings = ent
	return nil
}

func (f *fakeStore) UpdateUserSettings(ctx context.Context, ent UserSettingsUpdate) error { return nil }

func TestApplyTaskCreated(t *testing.T) {
	fs := &fakeStore{}
	data := struct {
		Title    string `json:"title"`
		Notes    string `json:"notes"`
		Category string `json:"category"`
		Order    int    `json:"order"`
	}{"title1", "note", "cat", 1}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskCreated, UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 1}
	Apply(context.Background(), fs, ev)
	if fs.upsertTask.PartitionKey != "u1" || fs.upsertTask.RowKey != "t1" || fs.upsertTask.Title != "title1" || fs.upsertTask.Order != 1 || fs.upsertTask.Timestamp != 1 {
		t.Fatalf("unexpected upsertTask: %#v", fs.upsertTask)
	}
}

func TestApplyTaskUpdated(t *testing.T) {
	fs := &fakeStore{}
	data := struct {
		Title *string `json:"title"`
		Notes *string `json:"notes"`
		Order *int    `json:"order"`
	}{Title: ptrString("new"), Notes: ptrString("n"), Order: ptrInt(5)}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskUpdated, UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 1}
	Apply(context.Background(), fs, ev)
	if fs.upsertTask.RowKey != "t1" || fs.upsertTask.Title != "new" || fs.upsertTask.Order != 5 || fs.upsertTask.Timestamp != 1 {
		t.Fatalf("unexpected upsertTask: %#v", fs.upsertTask)
	}
}

func TestApplyTaskCompleted(t *testing.T) {
	fs := &fakeStore{}
	ev := Event{Type: TaskCompleted, UserID: "u1", EntityID: "t1", Timestamp: 1}
	Apply(context.Background(), fs, ev)
	if fs.upsertTask.RowKey != "t1" || !fs.upsertTask.Done || fs.upsertTask.Timestamp != 1 {
		t.Fatalf("unexpected upsertTask: %#v", fs.upsertTask)
	}
}

func TestApplyTaskCompletedIgnoresOldEvent(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:    Entity{PartitionKey: "u1", RowKey: "t1"},
		Done:      false,
		DoneType:  EdmBoolean,
		Timestamp: 5,
	}}}
	ev := Event{Type: TaskCompleted, UserID: "u1", EntityID: "t1", Timestamp: 3}
	Apply(context.Background(), fs, ev)
	ent := fs.tasks["t1"]
	if ent.Done || ent.Timestamp != 5 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyUserCreated(t *testing.T) {
	fs := &fakeStore{}
	data := struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}{"Alice", "a@example.com"}
	payload, _ := json.Marshal(data)
	ev := Event{Type: UserCreated, EntityID: "u1", UserID: "u1", Data: payload}
	Apply(context.Background(), fs, ev)
	if fs.upsertUser.PartitionKey != "u1" || fs.upsertUser.Name != "Alice" {
		t.Fatalf("unexpected upsertUser: %#v", fs.upsertUser)
	}
}

func TestApplyUserSettingsUpdatedIgnoresOldEvent(t *testing.T) {
	fs := &fakeStore{settings: map[string]UserSettingsEntity{"u1": {
		Entity:               Entity{PartitionKey: "u1", RowKey: "u1"},
		TasksPerCategory:     10,
		TasksPerCategoryType: EdmInt32,
		ShowDoneTasks:        true,
		ShowDoneTasksType:    EdmBoolean,
		Timestamp:            5,
		TimestampType:        EdmInt64,
	}}}
	tpc := 3
	sdt := false
	data := UserSettingsUpdatedEventData{TasksPerCategory: &tpc, ShowDoneTasks: &sdt}
	payload, _ := json.Marshal(data)
	ev := Event{Type: UserSettingsUpdated, UserID: "u1", EntityID: "u1", Data: payload, Timestamp: 2}
	Apply(context.Background(), fs, ev)
	ent := fs.settings["u1"]
	if ent.TasksPerCategory != 10 || !ent.ShowDoneTasks || ent.Timestamp != 5 {
		t.Fatalf("unexpected settings entity: %#v", ent)
	}
}

func ptrString(s string) *string { return &s }
func ptrInt(i int) *int          { return &i }
