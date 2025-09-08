package domain

import (
	"context"
	"encoding/json"
	"testing"
)

func ptrString(s string) *string { return &s }
func ptrInt(i int) *int          { return &i }
func ptrBool(b bool) *bool       { return &b }

func TestTaskCreated(t *testing.T) {
	fs := &fakeStore{}
	tp := TaskProcessor{}
	data := TaskCreatedEventData{Title: "title1", Notes: "note", Category: "cat", Order: 1}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskCreated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 1}
	if err := tp.Handle(context.Background(), fs, ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if fs.insertTask.RowKey != "t1" || fs.insertTask.Title != "title1" || fs.insertTask.Order != 1 || fs.insertTask.EventTimestamp != 1 {
		t.Fatalf("unexpected insertTask: %#v", fs.insertTask)
	}
}

func TestTaskUpdated(t *testing.T) {
	fs := &fakeStore{}
	tp := TaskProcessor{}
	data := TaskUpdatedEventData{Title: ptrString("new"), Notes: ptrString("n"), Order: ptrInt(5)}
	payload, _ := json.Marshal(data)
	ev := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: payload, Timestamp: 1}
	if err := tp.Handle(context.Background(), fs, ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if fs.insertTask.RowKey != "t1" || fs.insertTask.Title != "new" || fs.insertTask.Order != 5 || fs.insertTask.EventTimestamp != 1 {
		t.Fatalf("unexpected insertTask: %#v", fs.insertTask)
	}
}

func TestTaskCompleted(t *testing.T) {
	fs := &fakeStore{}
	tp := TaskProcessor{}
	ev := Event{Type: TaskCompleted, EntityType: "task", UserID: "u1", EntityID: "t1", Timestamp: 1}
	if err := tp.Handle(context.Background(), fs, ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	ent, _ := fs.GetTask(context.Background(), "u1", "t1")
	if ent == nil || !ent.Done {
		t.Fatalf("expected stored task done true: %#v", ent)
	}
}

func TestTaskUpdateBeforeCreateMergesFields(t *testing.T) {
	fs := &fakeStore{}
	tp := TaskProcessor{}
	ctx := context.Background()

	// Process newer update first
	upd1 := TaskUpdatedEventData{Done: ptrBool(true)}
	b1, _ := json.Marshal(upd1)
	ev1 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b1, Timestamp: 3}
	if err := tp.Handle(ctx, fs, ev1); err != nil {
		t.Fatalf("handle1: %v", err)
	}

	// Stale update with notes
	upd2 := TaskUpdatedEventData{Notes: ptrString("note")}
	b2, _ := json.Marshal(upd2)
	ev2 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b2, Timestamp: 2}
	if err := tp.Handle(ctx, fs, ev2); err != nil {
		t.Fatalf("handle2: %v", err)
	}

	// Oldest create arrives last
	created := TaskCreatedEventData{Title: "t"}
	b3, _ := json.Marshal(created)
	ev3 := Event{Type: TaskCreated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b3, Timestamp: 1}
	if err := tp.Handle(ctx, fs, ev3); err != nil {
		t.Fatalf("handle3: %v", err)
	}

	ent, _ := fs.GetTask(ctx, "u1", "t1")
	if ent.Title != "t" || ent.Notes != "note" || !ent.Done {
		t.Fatalf("unexpected task: %#v", ent)
	}
}
