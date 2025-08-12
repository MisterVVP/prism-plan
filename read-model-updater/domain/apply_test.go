package domain

import (
	"context"
	"encoding/json"
	"testing"
)

type fakeStore struct {
	upsertTask map[string]any
	updateTask map[string]any
	donePK     string
	doneRK     string
	upsertUser map[string]any
}

func (f *fakeStore) UpsertTask(ctx context.Context, ent map[string]any) error {
	f.upsertTask = ent
	return nil
}

func (f *fakeStore) UpdateTask(ctx context.Context, ent map[string]any) error {
	f.updateTask = ent
	return nil
}

func (f *fakeStore) SetTaskDone(ctx context.Context, pk, rk string) error {
	f.donePK, f.doneRK = pk, rk
	return nil
}

func (f *fakeStore) UpsertUser(ctx context.Context, ent map[string]any) error {
	f.upsertUser = ent
	return nil
}

func TestApplyTaskCreated(t *testing.T) {
	fs := &fakeStore{}
	data := map[string]any{"title": "title1", "notes": "note", "category": "cat", "order": float64(1)}
	payload, _ := json.Marshal(data)
	ev := Event{Type: "task-created", UserID: "u1", EntityID: "t1", Data: payload}
	Apply(context.Background(), fs, ev)
	if fs.upsertTask["PartitionKey"] != "u1" || fs.upsertTask["RowKey"] != "t1" || fs.upsertTask["Title"] != "title1" {
		t.Fatalf("unexpected upsertTask: %#v", fs.upsertTask)
	}
}

func TestApplyTaskUpdated(t *testing.T) {
	fs := &fakeStore{}
	data := map[string]any{"title": "new", "notes": "n"}
	payload, _ := json.Marshal(data)
	ev := Event{Type: "task-updated", UserID: "u1", EntityID: "t1", Data: payload}
	Apply(context.Background(), fs, ev)
	if fs.updateTask["Title"] != "new" || fs.updateTask["PartitionKey"] != "u1" || fs.updateTask["RowKey"] != "t1" {
		t.Fatalf("unexpected updateTask: %#v", fs.updateTask)
	}
}

func TestApplyTaskCompleted(t *testing.T) {
	fs := &fakeStore{}
	ev := Event{Type: "task-completed", UserID: "u1", EntityID: "t1"}
	Apply(context.Background(), fs, ev)
	if fs.donePK != "u1" || fs.doneRK != "t1" {
		t.Fatalf("expected pk/rk u1 t1, got %s %s", fs.donePK, fs.doneRK)
	}
}

func TestApplyUserCreated(t *testing.T) {
	fs := &fakeStore{}
	data := map[string]any{"name": "Alice", "email": "a@example.com"}
	payload, _ := json.Marshal(data)
	ev := Event{Type: "user-created", EntityID: "u1", UserID: "u1", Data: payload}
	Apply(context.Background(), fs, ev)
	if fs.upsertUser["PartitionKey"] != "u1" || fs.upsertUser["Name"] != "Alice" {
		t.Fatalf("unexpected upsertUser: %#v", fs.upsertUser)
	}
}
