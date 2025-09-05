package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeStore struct {
	tasks          map[string]TaskEntity
	settings       map[string]UserSettingsEntity
	upsertTask     TaskEntity
	insertTask     TaskEntity
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

func (f *fakeStore) InsertTask(ctx context.Context, ent TaskEntity) error {
	if f.tasks == nil {
		f.tasks = map[string]TaskEntity{}
	}
	if _, exists := f.tasks[ent.RowKey]; exists {
		return errors.New("conflict")
	}
	f.tasks[ent.RowKey] = ent
	f.insertTask = ent
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
		ent.Order = *upd.Order
		if upd.OrderType != nil {
			ent.OrderType = *upd.OrderType
		}
	}
	if upd.Done != nil {
		ent.Done = *upd.Done
		if upd.DoneType != nil {
			ent.DoneType = *upd.DoneType
		}
	}
	if upd.EventTimestamp != nil {
		ent.EventTimestamp = *upd.EventTimestamp
		if upd.EventTimestampType != nil {
			ent.EventTimestampType = *upd.EventTimestampType
		}
	}
	f.tasks[upd.RowKey] = ent
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
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fs.insertTask.PartitionKey != "u1" || fs.insertTask.RowKey != "t1" || fs.insertTask.Title != "title1" || fs.insertTask.Order != 1 || fs.insertTask.EventTimestamp != 1 {
		t.Fatalf("unexpected insertTask: %#v", fs.insertTask)
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
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fs.upsertTask.RowKey != "t1" || fs.upsertTask.Title != "new" || fs.upsertTask.Order != 5 || fs.upsertTask.EventTimestamp != 1 {
		t.Fatalf("unexpected upsertTask: %#v", fs.upsertTask)
	}
}

func TestApplyTaskCompleted(t *testing.T) {
	fs := &fakeStore{}
	ev := Event{Type: TaskCompleted, UserID: "u1", EntityID: "t1", Timestamp: 1}
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fs.upsertTask.RowKey != "t1" || !fs.upsertTask.Done || fs.upsertTask.EventTimestamp != 1 {
		t.Fatalf("unexpected upsertTask: %#v", fs.upsertTask)
	}
}

func TestApplyTaskCompletedIgnoresOldEvent(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Done:           false,
		DoneType:       EdmBoolean,
		EventTimestamp: 5,
	}}}
	ev := Event{Type: TaskCompleted, UserID: "u1", EntityID: "t1", Timestamp: 3}
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ent := fs.tasks["t1"]
	if ent.Done || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyTaskUpdatedStaleEventDoesNotOverride(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Title:          "a",
		Order:          10,
		OrderType:      EdmInt32,
		Done:           true,
		DoneType:       EdmBoolean,
		EventTimestamp: 5,
	}}}
	done := false
	order := 0
	data := TaskUpdatedEventData{Done: &done, Order: &order}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskUpdated, UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 3}
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ent := fs.tasks["t1"]
	if !ent.Done || ent.Order != 10 || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyTaskUpdatedMergesStaleFields(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Title:          "a",
		Order:          10,
		OrderType:      EdmInt32,
		Done:           true,
		DoneType:       EdmBoolean,
		EventTimestamp: 5,
	}}}
	notes := "note"
	data := TaskUpdatedEventData{Notes: &notes}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskUpdated, UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 3}
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ent := fs.tasks["t1"]
	if ent.Notes != "note" || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyTaskUpdatedMergesStaleDone(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Done:           false,
		DoneType:       EdmBoolean,
		EventTimestamp: 5,
	}}}
	done := true
	data := TaskUpdatedEventData{Done: &done}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskUpdated, UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 3}
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ent := fs.tasks["t1"]
	if !ent.Done || ent.EventTimestamp != 5 {
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
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
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
		EventTimestamp:       5,
		EventTimestampType:   EdmInt64,
	}}}
	tpc := 3
	sdt := false
	data := UserSettingsUpdatedEventData{TasksPerCategory: &tpc, ShowDoneTasks: &sdt}
	payload, _ := json.Marshal(data)
	ev := Event{Type: UserSettingsUpdated, UserID: "u1", EntityID: "u1", Data: payload, Timestamp: 2}
	if err := Apply(context.Background(), fs, ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ent := fs.settings["u1"]
	if ent.TasksPerCategory != 10 || !ent.ShowDoneTasks || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected settings entity: %#v", ent)
	}
}

func ptrString(s string) *string { return &s }
func ptrInt(i int) *int          { return &i }
