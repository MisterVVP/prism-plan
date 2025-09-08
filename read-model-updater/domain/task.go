package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// TaskProcessor handles task-related events.
type TaskProcessor struct{}

// taskFields represents optional task fields used for creation and updates.
type taskFields struct {
	Title    *string
	Notes    *string
	Category *string
	Order    *int
	Done     *bool
}

// ensureTask loads an existing task or creates a new skeleton populated with provided fields.
func ensureTask(ctx context.Context, st Storage, pk, rk string, ts int64, f taskFields) (*TaskEntity, bool, error) {
	ent, err := st.GetTask(ctx, pk, rk)
	if err != nil {
		return nil, false, err
	}
	if ent != nil {
		return ent, false, nil
	}
	ent = &TaskEntity{Entity: Entity{PartitionKey: pk, RowKey: rk}, EventTimestamp: ts, EventTimestampType: EdmInt64}
	if f.Title != nil {
		ent.Title = *f.Title
	}
	if f.Notes != nil {
		ent.Notes = *f.Notes
	}
	if f.Category != nil {
		ent.Category = *f.Category
	}
	if f.Order != nil {
		ent.Order = *f.Order
		ent.OrderType = EdmInt32
	}
	if f.Done != nil {
		ent.Done = *f.Done
		ent.DoneType = EdmBoolean
	}
	if err := st.InsertTask(ctx, *ent); err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 409 {
			ent, err = st.GetTask(ctx, pk, rk)
			if err != nil {
				return nil, false, err
			}
			return ent, false, nil
		}
		return nil, false, err
	}
	return ent, true, nil
}

// mergeTask merges fields into ent respecting timestamps.
func mergeTask(ent *TaskEntity, ts int64, f taskFields) (TaskUpdate, bool, error) {
	if ts == ent.EventTimestamp {
		return TaskUpdate{}, false, fmt.Errorf("task %s received event with identical timestamp", ent.RowKey)
	}
	upd := TaskUpdate{Entity: ent.Entity}
	changed := false
	if ts > ent.EventTimestamp {
		if f.Title != nil {
			upd.Title = f.Title
			ent.Title = *f.Title
		}
		if f.Notes != nil {
			upd.Notes = f.Notes
			ent.Notes = *f.Notes
		}
		if f.Category != nil {
			upd.Category = f.Category
			ent.Category = *f.Category
		}
		if f.Order != nil {
			upd.Order = f.Order
			t := EdmInt32
			upd.OrderType = &t
			ent.Order = *f.Order
			ent.OrderType = EdmInt32
		}
		if f.Done != nil {
			upd.Done = f.Done
			t := EdmBoolean
			upd.DoneType = &t
			ent.Done = *f.Done
			ent.DoneType = EdmBoolean
		}
		upd.EventTimestamp = &ts
		t := EdmInt64
		upd.EventTimestampType = &t
		ent.EventTimestamp = ts
		ent.EventTimestampType = EdmInt64
		changed = upd.Title != nil || upd.Notes != nil || upd.Category != nil || upd.Order != nil || upd.Done != nil
	} else {
		if f.Title != nil && ent.Title == "" {
			upd.Title = f.Title
			ent.Title = *f.Title
			changed = true
		}
		if f.Notes != nil && ent.Notes == "" {
			upd.Notes = f.Notes
			ent.Notes = *f.Notes
			changed = true
		}
		if f.Category != nil && ent.Category == "" {
			upd.Category = f.Category
			ent.Category = *f.Category
			changed = true
		}
		if f.Order != nil && ent.OrderType == "" {
			upd.Order = f.Order
			t := EdmInt32
			upd.OrderType = &t
			ent.Order = *f.Order
			ent.OrderType = EdmInt32
			changed = true
		}
		if f.Done != nil && ent.DoneType == "" {
			upd.Done = f.Done
			t := EdmBoolean
			upd.DoneType = &t
			ent.Done = *f.Done
			ent.DoneType = EdmBoolean
			changed = true
		}
	}
	if upd.EventTimestamp != nil || changed {
		return upd, true, nil
	}
	return TaskUpdate{}, false, nil
}

// Handle processes a task event based on its type.
func (TaskProcessor) Handle(ctx context.Context, st Storage, ev Event) error {
	pk := ev.UserID
	rk := ev.EntityID
	switch ev.Type {
	case TaskCreated:
		var data TaskCreatedEventData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return err
		}
		f := taskFields{Title: &data.Title, Notes: &data.Notes, Category: &data.Category, Order: &data.Order}
		ent, created, err := ensureTask(ctx, st, pk, rk, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if created {
			return nil
		}
		upd, changed, err := mergeTask(ent, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if changed {
			return st.UpdateTask(ctx, upd)
		}
		return nil
	case TaskUpdated:
		var data TaskUpdatedEventData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return err
		}
		f := taskFields{Title: data.Title, Notes: data.Notes, Category: data.Category, Order: data.Order, Done: data.Done}
		ent, created, err := ensureTask(ctx, st, pk, rk, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if created {
			return nil
		}
		upd, changed, err := mergeTask(ent, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if changed {
			return st.UpdateTask(ctx, upd)
		}
		return nil
	case TaskCompleted:
		done := true
		f := taskFields{Done: &done}
		ent, created, err := ensureTask(ctx, st, pk, rk, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if created {
			return nil
		}
		upd, changed, err := mergeTask(ent, ev.Timestamp, f)
		if err != nil {
			return err
		}
		if changed {
			return st.UpdateTask(ctx, upd)
		}
		return nil
	}
	return nil
}
