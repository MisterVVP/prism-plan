package domain

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeStore struct {
	upsertTask TaskEntity
	updateTask TaskUpdate
	donePK     string
	doneRK     string
	upsertUser UserEntity
}

func (f *fakeStore) UpsertTask(ctx context.Context, ent TaskEntity) error {
	f.upsertTask = ent
	return nil
}

func (f *fakeStore) UpdateTask(ctx context.Context, ent TaskUpdate) error {
	f.updateTask = ent
	return nil
}

func (f *fakeStore) SetTaskDone(ctx context.Context, pk, rk string) error {
	f.donePK, f.doneRK = pk, rk
	return nil
}

func (f *fakeStore) UpsertUser(ctx context.Context, ent UserEntity) error {
	f.upsertUser = ent
	return nil
}

func TestApplyTaskCreated(t *testing.T) {
	fs := &fakeStore{}
	data := struct {
		Title    string `json:"title"`
		Notes    string `json:"notes"`
		Category string `json:"category"`
		Order    int    `json:"order"`
	}{"title1", "note", "cat", 1}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskCreated, UserID: "u1", EntityID: "t1", Data: payload}
	Apply(context.Background(), fs, ev)
	if fs.upsertTask.PartitionKey != "u1" || fs.upsertTask.RowKey != "t1" || fs.upsertTask.Title != "title1" || fs.upsertTask.Order != 1 {
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
	ev := Event{Type: TaskUpdated, UserID: "u1", EntityID: "t1", Data: payload}
	Apply(context.Background(), fs, ev)
	if fs.updateTask.PartitionKey != "u1" || fs.updateTask.RowKey != "t1" || fs.updateTask.Title == nil || *fs.updateTask.Title != "new" || fs.updateTask.Order == nil || *fs.updateTask.Order != 5 {
		t.Fatalf("unexpected updateTask: %#v", fs.updateTask)
	}
}

func TestApplyTaskCompleted(t *testing.T) {
	fs := &fakeStore{}
	ev := Event{Type: TaskCompleted, UserID: "u1", EntityID: "t1"}
	Apply(context.Background(), fs, ev)
	if fs.donePK != "u1" || fs.doneRK != "t1" {
		t.Fatalf("expected pk/rk u1 t1, got %s %s", fs.donePK, fs.doneRK)
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

func ptrString(s string) *string { return &s }
func ptrInt(i int) *int          { return &i }
