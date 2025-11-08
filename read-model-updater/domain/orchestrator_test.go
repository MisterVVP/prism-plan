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
		return errors.New("conflict")
	}
	f.tasks[ent.RowKey] = ent
	f.insertTask = ent
	return nil
}

func (f *fakeStore) UpdateTask(ctx context.Context, upd TaskUpdate, _ string) error {
	if f.tasks == nil {
		f.tasks = map[string]TaskEntity{}
	}
	f.updateTask = upd
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
	}
	if upd.Done != nil {
		ent.Done = *upd.Done
	}
	if upd.EventTimestamp != nil {
		ent.EventTimestamp = *upd.EventTimestamp
	}
	f.tasks[upd.RowKey] = ent
	return nil
}

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
	}
	if ent.ShowDoneTasks != nil {
		cur.ShowDoneTasks = *ent.ShowDoneTasks
	}
	if ent.EventTimestamp != nil {
		cur.EventTimestamp = *ent.EventTimestamp
	}
	f.settings[ent.RowKey] = cur
	f.updateSettings = ent
	return nil
}

func TestApplyTaskCreated(t *testing.T) {
	fs := &fakeStore{}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	data := struct {
		Title    string `json:"title"`
		Notes    string `json:"notes"`
		Category string `json:"category"`
		Order    int    `json:"order"`
	}{"title1", "note", "cat", 1}
	payload, _ := json.Marshal(data)
	ev := Event{EntityType: "task", Type: TaskCreated, UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 1}
	if err := orch.Apply(context.Background(), ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fs.insertTask.PartitionKey != "u1" || fs.insertTask.RowKey != "t1" || fs.insertTask.Title != "title1" || fs.insertTask.Order != 1 || fs.insertTask.EventTimestamp != 1 {
		t.Fatalf("unexpected insertTask: %#v", fs.insertTask)
	}
}

func TestApplyTaskUpdatedMissingTask(t *testing.T) {
	fs := &fakeStore{}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "task", Type: TaskUpdated, UserID: "u1", EntityID: "t1", Timestamp: 1}
	if err := orch.Apply(context.Background(), ev); err == nil {
		t.Fatalf("expected error for missing task")
	}
}

func TestApplyTaskCompletedMissingTask(t *testing.T) {
	fs := &fakeStore{}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "task", Type: TaskCompleted, UserID: "u1", EntityID: "t1", Timestamp: 1}
	if err := orch.Apply(context.Background(), ev); err == nil {
		t.Fatalf("expected error for missing task")
	}
}

func TestApplyTaskCompletedStaleEventReturnsError(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Done:           false,
		EventTimestamp: 5,
	}}}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "task", Type: TaskCompleted, UserID: "u1", EntityID: "t1", Timestamp: 3}
	if err := orch.Apply(context.Background(), ev); err == nil {
		t.Fatalf("expected error for stale completion")
	}
	ent := fs.tasks["t1"]
	if ent.Done || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyTaskReopenedMissingTask(t *testing.T) {
	fs := &fakeStore{}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "task", Type: TaskReopened, UserID: "u1", EntityID: "t1", Timestamp: 1}
	if err := orch.Apply(context.Background(), ev); err == nil {
		t.Fatalf("expected error for missing task")
	}
}

func TestApplyTaskReopenedStaleEventReturnsError(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Done:           false,
		EventTimestamp: 5,
	}}}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "task", Type: TaskReopened, UserID: "u1", EntityID: "t1", Timestamp: 3}
	if err := orch.Apply(context.Background(), ev); err == nil {
		t.Fatalf("expected error for stale reopen")
	}
	ent := fs.tasks["t1"]
	if ent.Done || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyTaskReopened(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Done:           true,
		EventTimestamp: 5,
	}}}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "task", Type: TaskReopened, UserID: "u1", EntityID: "t1", Timestamp: 6}
	if err := orch.Apply(context.Background(), ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ent := fs.tasks["t1"]
	if ent.Done || ent.EventTimestamp != 6 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyTaskUpdatedStaleEventReturnsError(t *testing.T) {
	fs := &fakeStore{tasks: map[string]TaskEntity{"t1": {
		Entity:         Entity{PartitionKey: "u1", RowKey: "t1"},
		Title:          "a",
		Order:          10,
		Done:           true,
		EventTimestamp: 5,
	}}}
	done := false
	order := 0
	data := TaskUpdatedEventData{Done: &done, Order: &order}
	payload, _ := json.Marshal(data)
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "task", Type: TaskUpdated, UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 3}
	if err := orch.Apply(context.Background(), ev); err == nil {
		t.Fatalf("expected error for stale update")
	}
	ent := fs.tasks["t1"]
	if !ent.Done || ent.Order != 10 || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected task entity: %#v", ent)
	}
}

func TestApplyUserCreated(t *testing.T) {
	fs := &fakeStore{}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	data := struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}{"Alice", "a@example.com"}
	payload, _ := json.Marshal(data)
	ev := Event{EntityType: "user", Type: UserCreated, EntityID: "u1", UserID: "u1", Data: payload}
	if err := orch.Apply(context.Background(), ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fs.upsertUser.PartitionKey != "u1" || fs.upsertUser.Name != "Alice" {
		t.Fatalf("unexpected upsertUser: %#v", fs.upsertUser)
	}
}

func TestApplyUserSettingsUpdatedStaleEventReturnsError(t *testing.T) {
	fs := &fakeStore{settings: map[string]UserSettingsEntity{"u1": {
		Entity:           Entity{PartitionKey: "u1", RowKey: "u1"},
		TasksPerCategory: 10,
		ShowDoneTasks:    true,
		EventTimestamp:   5,
	}}}
	tpc := 3
	sdt := false
	data := UserSettingsUpdatedEventData{TasksPerCategory: &tpc, ShowDoneTasks: &sdt}
	payload, _ := json.Marshal(data)
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "user-settings", Type: UserSettingsUpdated, UserID: "u1", EntityID: "u1", Data: payload, Timestamp: 2}
	if err := orch.Apply(context.Background(), ev); err == nil {
		t.Fatalf("expected error for stale settings update")
	}
	ent := fs.settings["u1"]
	if ent.TasksPerCategory != 10 || !ent.ShowDoneTasks || ent.EventTimestamp != 5 {
		t.Fatalf("unexpected settings entity: %#v", ent)
	}
}

func TestApplyUserSettingsUpdatedUpdatesExisting(t *testing.T) {
	fs := &fakeStore{settings: map[string]UserSettingsEntity{
		"u1": {
			Entity:           Entity{PartitionKey: "u1", RowKey: "u1"},
			TasksPerCategory: 3,
			ShowDoneTasks:    false,
			EventTimestamp:   1,
		},
	}}
	sdt := true
	data := UserSettingsUpdatedEventData{ShowDoneTasks: &sdt}
	payload, _ := json.Marshal(data)
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	ev := Event{EntityType: "user-settings", Type: UserSettingsUpdated, UserID: "u1", EntityID: "u1", Data: payload, Timestamp: 2}
	if err := orch.Apply(context.Background(), ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if fs.updateSettings.PartitionKey != "u1" || fs.updateSettings.ShowDoneTasks == nil || !*fs.updateSettings.ShowDoneTasks {
		t.Fatalf("unexpected update payload: %#v", fs.updateSettings)
	}
	if fs.settings["u1"].ShowDoneTasks != true || fs.settings["u1"].EventTimestamp != 2 {
		t.Fatalf("settings not updated: %#v", fs.settings["u1"])
	}
}

func TestApplyUserSettingsUpdatedCreatesWhenMissing(t *testing.T) {
	fs := &fakeStore{}
	orch := NewOrchestrator(NewTaskService(fs), NewUserService(fs))
	sdt := true
	data := UserSettingsUpdatedEventData{ShowDoneTasks: &sdt}
	payload, _ := json.Marshal(data)
	ev := Event{EntityType: "user-settings", Type: UserSettingsUpdated, UserID: "u1", EntityID: "u1", Data: payload, Timestamp: 1}
	if err := orch.Apply(context.Background(), ev); err != nil {
		t.Fatalf("apply: %v", err)
	}
	ent := fs.settings["u1"]
	if !ent.ShowDoneTasks || ent.EventTimestamp != 1 {
		t.Fatalf("settings not created: %#v", ent)
	}
}

func ptrString(s string) *string { return &s }
func ptrInt(i int) *int          { return &i }
