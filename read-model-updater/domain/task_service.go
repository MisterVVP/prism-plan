package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// TaskStorage defines methods required for updating task read models.
type TaskStorage interface {
	GetTask(ctx context.Context, pk, rk string) (*TaskEntity, error)
	InsertTask(ctx context.Context, ent TaskEntity) error
	UpdateTask(ctx context.Context, ent TaskUpdate, etag string) error
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
			Entity:         Entity{PartitionKey: pk, RowKey: rk},
			Title:          eventData.Title,
			Notes:          eventData.Notes,
			Category:       eventData.Category,
			Order:          eventData.Order,
			Done:           false,
			EventTimestamp: ev.Timestamp,
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
		}
		if eventData.Done != nil {
			upd.Done = eventData.Done
		}
		upd.EventTimestamp = &ev.Timestamp
		if upd.Title == nil && upd.Notes == nil && upd.Category == nil && upd.Order == nil && upd.Done == nil {
			return fmt.Errorf("task %s update had no fields", rk)
		}
		for {
			if ev.Timestamp <= ent.EventTimestamp {
				log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-updated event")
				return fmt.Errorf("task %s received stale update", rk)
			}
			if err := s.st.UpdateTask(ctx, upd, ent.ETag); err != nil {
				if !errors.Is(err, ErrConcurrencyConflict) {
					return err
				}
				ent, err = s.st.GetTask(ctx, pk, rk)
				if err != nil {
					return err
				}
				if ent == nil {
					log.WithField("task", rk).Error("task-updated event lost entity during retry")
					return fmt.Errorf("task %s not found", rk)
				}
				continue
			}
			return nil
		}
	case TaskCompleted:
		ent, err := s.st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			log.WithField("task", rk).Error("task-completed event for missing task")
			return fmt.Errorf("task %s not found", rk)
		}
		done := true
		ts := ev.Timestamp
		upd := TaskUpdate{
			Entity:         Entity{PartitionKey: pk, RowKey: rk},
			Done:           &done,
			EventTimestamp: &ts,
		}
		for {
			if ev.Timestamp <= ent.EventTimestamp {
				log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-completed event")
				return fmt.Errorf("task %s received stale completion", rk)
			}
			if err := s.st.UpdateTask(ctx, upd, ent.ETag); err != nil {
				if !errors.Is(err, ErrConcurrencyConflict) {
					return err
				}
				ent, err = s.st.GetTask(ctx, pk, rk)
				if err != nil {
					return err
				}
				if ent == nil {
					log.WithField("task", rk).Error("task-completed retry lost entity")
					return fmt.Errorf("task %s not found", rk)
				}
				continue
			}
			return nil
		}
	case TaskReopened:
		ent, err := s.st.GetTask(ctx, pk, rk)
		if err != nil {
			return err
		}
		if ent == nil {
			log.WithField("task", rk).Error("task-reopened event for missing task")
			return fmt.Errorf("task %s not found", rk)
		}
		done := false
		ts := ev.Timestamp
		upd := TaskUpdate{
			Entity:         Entity{PartitionKey: pk, RowKey: rk},
			Done:           &done,
			EventTimestamp: &ts,
		}
		for {
			if ev.Timestamp <= ent.EventTimestamp {
				log.WithFields(log.Fields{"task": rk, "ts": ev.Timestamp, "current": ent.EventTimestamp}).Error("stale task-reopened event")
				return fmt.Errorf("task %s received stale reopen", rk)
			}
			if err := s.st.UpdateTask(ctx, upd, ent.ETag); err != nil {
				if !errors.Is(err, ErrConcurrencyConflict) {
					return err
				}
				ent, err = s.st.GetTask(ctx, pk, rk)
				if err != nil {
					return err
				}
				if ent == nil {
					log.WithField("task", rk).Error("task-reopened retry lost entity")
					return fmt.Errorf("task %s not found", rk)
				}
				continue
			}
			return nil
		}
	default:
		return fmt.Errorf("unknown task event %s", ev.Type)
	}
}
