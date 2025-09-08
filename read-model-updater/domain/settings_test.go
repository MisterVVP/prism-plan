package domain

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSettingsCreated(t *testing.T) {
	fs := &fakeStore{}
	sp := SettingsProcessor{}
	data := UserSettingsEventData{TasksPerCategory: 5, ShowDoneTasks: true}
	b, _ := json.Marshal(data)
	ev := Event{Type: UserSettingsCreated, EntityType: "user-settings", EntityID: "u1", Data: b, Timestamp: 1}
	if err := sp.Handle(context.Background(), fs, ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if fs.settings["u1"].TasksPerCategory != 5 || !fs.settings["u1"].ShowDoneTasks {
		t.Fatalf("unexpected settings: %#v", fs.settings["u1"])
	}
}

func TestSettingsUpdated(t *testing.T) {
	fs := &fakeStore{}
	sp := SettingsProcessor{}
	data := UserSettingsUpdatedEventData{ShowDoneTasks: ptrBool(true)}
	b, _ := json.Marshal(data)
	ev := Event{Type: UserSettingsUpdated, EntityType: "user-settings", EntityID: "u1", Data: b, Timestamp: 1}
	if err := sp.Handle(context.Background(), fs, ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !fs.settings["u1"].ShowDoneTasks {
		t.Fatalf("expected showDoneTasks true: %#v", fs.settings["u1"])
	}
}

func TestSettingsUpdateBeforeCreateMergesFields(t *testing.T) {
	fs := &fakeStore{}
	sp := SettingsProcessor{}
	ctx := context.Background()

	upd := UserSettingsUpdatedEventData{ShowDoneTasks: ptrBool(true)}
	b1, _ := json.Marshal(upd)
	ev1 := Event{Type: UserSettingsUpdated, EntityType: "user-settings", EntityID: "u1", Data: b1, Timestamp: 2}
	if err := sp.Handle(ctx, fs, ev1); err != nil {
		t.Fatalf("handle1: %v", err)
	}

	create := UserSettingsEventData{TasksPerCategory: 3}
	b2, _ := json.Marshal(create)
	ev2 := Event{Type: UserSettingsCreated, EntityType: "user-settings", EntityID: "u1", Data: b2, Timestamp: 1}
	if err := sp.Handle(ctx, fs, ev2); err != nil {
		t.Fatalf("handle2: %v", err)
	}

	ent := fs.settings["u1"]
	if ent.TasksPerCategory != 3 || !ent.ShowDoneTasks {
		t.Fatalf("unexpected settings: %#v", ent)
	}
}
