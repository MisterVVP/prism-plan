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
	if fs.insertTask.RowKey != "t1" || fs.insertTask.Title != "title1" || fs.insertTask.Order == nil || *fs.insertTask.Order != 1 || fs.insertTask.EventTimestamp != 1 {
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
	if fs.insertTask.RowKey != "t1" || fs.insertTask.Title != "new" || fs.insertTask.Order == nil || *fs.insertTask.Order != 5 || fs.insertTask.EventTimestamp != 1 {
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
	if ent == nil || ent.Done == nil || !*ent.Done {
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
	if ent.Title != "t" || ent.Notes != "note" || ent.Done == nil || !*ent.Done {
		t.Fatalf("unexpected task: %#v", ent)
	}
}

func TestTaskReopenIgnoresStaleCompletion(t *testing.T) {
	fs := &fakeStore{}
	tp := TaskProcessor{}
	ctx := context.Background()

	// create task
	created := TaskCreatedEventData{Title: "t", Category: "c", Order: 1}
	b1, _ := json.Marshal(created)
	ev1 := Event{Type: TaskCreated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b1, Timestamp: 1}
	if err := tp.Handle(ctx, fs, ev1); err != nil {
		t.Fatalf("handle create: %v", err)
	}

	// complete task
	done := TaskUpdatedEventData{Done: ptrBool(true), Category: ptrString("done")}
	b2, _ := json.Marshal(done)
	ev2 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b2, Timestamp: 2}
	if err := tp.Handle(ctx, fs, ev2); err != nil {
		t.Fatalf("handle complete: %v", err)
	}

	// reopen with later timestamp
	reopen := TaskUpdatedEventData{Done: ptrBool(false), Category: ptrString("c"), Order: ptrInt(1)}
	b3, _ := json.Marshal(reopen)
	ev3 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b3, Timestamp: 3}
	if err := tp.Handle(ctx, fs, ev3); err != nil {
		t.Fatalf("handle reopen: %v", err)
	}

	// stale completion arrives late
	stale := TaskUpdatedEventData{Done: ptrBool(true)}
	b4, _ := json.Marshal(stale)
	ev4 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b4, Timestamp: 2}
	if err := tp.Handle(ctx, fs, ev4); err != nil {
		t.Fatalf("handle stale: %v", err)
	}

	ent, _ := fs.GetTask(ctx, "u1", "t1")
	if ent == nil || ent.Done == nil || *ent.Done || ent.Category != "c" {
		t.Fatalf("unexpected task after reopen: %#v", ent)
	}
}

func TestTaskDuplicateEventNoChange(t *testing.T) {
	fs := &fakeStore{}
	tp := TaskProcessor{}
	ctx := context.Background()

	created := TaskCreatedEventData{Title: "t"}
	b, _ := json.Marshal(created)
	ev := Event{Type: TaskCreated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b, Timestamp: 1}
	if err := tp.Handle(ctx, fs, ev); err != nil {
		t.Fatalf("handle create: %v", err)
	}
	// process duplicate event
	if err := tp.Handle(ctx, fs, ev); err != nil {
		t.Fatalf("handle duplicate: %v", err)
	}
	ent, _ := fs.GetTask(ctx, "u1", "t1")
	if ent.Title != "t" {
		t.Fatalf("expected title t, got %#v", ent)
	}
}

func TestTaskReopenThenCompleteAgain(t *testing.T) {
	fs := &fakeStore{}
	tp := TaskProcessor{}
	ctx := context.Background()

	created := TaskCreatedEventData{Title: "t", Category: "c"}
	b1, _ := json.Marshal(created)
	ev1 := Event{Type: TaskCreated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b1, Timestamp: 1}
	if err := tp.Handle(ctx, fs, ev1); err != nil {
		t.Fatalf("handle create: %v", err)
	}

	done := TaskUpdatedEventData{Done: ptrBool(true), Category: ptrString("done")}
	b2, _ := json.Marshal(done)
	ev2 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b2, Timestamp: 2}
	if err := tp.Handle(ctx, fs, ev2); err != nil {
		t.Fatalf("handle done: %v", err)
	}

	reopen := TaskUpdatedEventData{Done: ptrBool(false), Category: ptrString("c")}
	b3, _ := json.Marshal(reopen)
	ev3 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b3, Timestamp: 3}
	if err := tp.Handle(ctx, fs, ev3); err != nil {
		t.Fatalf("handle reopen: %v", err)
	}

	doneAgain := TaskUpdatedEventData{Done: ptrBool(true), Category: ptrString("done")}
	b4, _ := json.Marshal(doneAgain)
	ev4 := Event{Type: TaskUpdated, EntityType: "task", UserID: "u1", EntityID: "t1", Data: b4, Timestamp: 4}
	if err := tp.Handle(ctx, fs, ev4); err != nil {
		t.Fatalf("handle done again: %v", err)
	}

	ent, _ := fs.GetTask(ctx, "u1", "t1")
	if ent == nil || ent.Done == nil || !*ent.Done || ent.Category != "done" {
		t.Fatalf("unexpected task after second completion: %#v", ent)
	}
}
