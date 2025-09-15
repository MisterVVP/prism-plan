package domain

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// TaskStorage defines methods required for updating task read models.
type TaskStorage interface {
	GetTask(ctx context.Context, pk, rk string) (*TaskEntity, error)
	InsertTask(ctx context.Context, ent TaskEntity) error
	UpdateTask(ctx context.Context, ent TaskUpdate) error
}

// TaskService processes task events.
type TaskService struct{ st TaskStorage }

func NewTaskService(st TaskStorage) TaskService { return TaskService{st: st} }

// Apply updates the read model for task related events.
func (s TaskService) Apply(ctx context.Context, ev Event) error {
	pk := ev.UserID
	rk := ev.EntityID
	switch ev.Type {
	case TaskCreated:
		var eventData TaskCreatedEventData
		if err := json.Unmarshal(ev.Data, &eventData); err != nil {
			return err
		}
		ent, err := s.st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent != nil {
			log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("duplicate task-created event")
			return fmt.Errorf("task %s already exists", rk)
		}
		ent = &TaskEntity{
			Entity:             Entity{PartitionKey: pk, RowKey: rk},
			Title:              eventData.Title,
			Notes:              eventData.Notes,
			Category:           eventData.Category,
			Order:              eventData.Order,
			OrderType:          EdmInt32,
			Done:               false,
			DoneType:           EdmBoolean,
			EventTimestamp:     ev.Timestamp,
			EventTimestampType: EdmInt64,
		}
		return s.st.InsertTask(ctx, *ent)
	case TaskUpdated:
		var eventData TaskUpdatedEventData
		if err := json.Unmarshal(ev.Data, &eventData); err != nil {
			return err
		}
		ent, err := s.st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			log.WithField("task", rk).Error("task-updated event for missing task")
			return fmt.Errorf("task %s not found", rk)
		}
		if ev.Timestamp <= ent.EventTimestamp {
			log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-updated event")
			return fmt.Errorf("task %s received stale update", rk)
		}
		upd := TaskUpdate{Entity: Entity{PartitionKey: pk, RowKey: rk}}
		if eventData.Title != nil {
			upd.Title = eventData.Title
		}
		if eventData.Notes != nil {
			upd.Notes = eventData.Notes
		}
		if eventData.Category != nil {
			upd.Category = eventData.Category
		}
		if eventData.Order != nil {
			upd.Order = eventData.Order
			t := EdmInt32
			upd.OrderType = &t
		}
		if eventData.Done != nil {
			upd.Done = eventData.Done
			t := EdmBoolean
			upd.DoneType = &t
		}
		upd.EventTimestamp = &ev.Timestamp
		t := EdmInt64
		upd.EventTimestampType = &t
		if upd.Title != nil || upd.Notes != nil || upd.Category != nil || upd.Order != nil || upd.Done != nil {
			return s.st.UpdateTask(ctx, upd)
		}
		return fmt.Errorf("task %s update had no fields", rk)
	case TaskCompleted:
		ent, err := s.st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			log.WithField("task", rk).Error("task-completed event for missing task")
			return fmt.Errorf("task %s not found", rk)
		}
		if ev.Timestamp <= ent.EventTimestamp {
			log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-completed event")
			return fmt.Errorf("task %s received stale completion", rk)
		}
		done := true
		dt := EdmBoolean
		ts := ev.Timestamp
		tp := EdmInt64
		upd := TaskUpdate{
			Entity:             Entity{PartitionKey: pk, RowKey: rk},
			Done:               &done,
			DoneType:           &dt,
			EventTimestamp:     &ts,
			EventTimestampType: &tp,
		}
		return s.st.UpdateTask(ctx, upd)
	case TaskReopened:
		ent, err := s.st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			log.WithField("task", rk).Error("task-reopened event for missing task")
			return fmt.Errorf("task %s not found", rk)
		}
		if ev.Timestamp <= ent.EventTimestamp {
			log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-reopened event")
			return fmt.Errorf("task %s received stale reopen", rk)
		}
		done := false
		dt := EdmBoolean
		ts := ev.Timestamp
		tp := EdmInt64
		upd := TaskUpdate{
			Entity:             Entity{PartitionKey: pk, RowKey: rk},
			Done:               &done,
			DoneType:           &dt,
			EventTimestamp:     &ts,
			EventTimestampType: &tp,
		}
		return s.st.UpdateTask(ctx, upd)
	default:
		return fmt.Errorf("unknown task event %s", ev.Type)
	}
}
