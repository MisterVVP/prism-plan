package main

import (
	"encoding/json"
	"sort"
)

// Task represents a single board item reconstructed from events.
type Task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Notes    string `json:"notes,omitempty"`
	Category string `json:"category"`
	Order    int    `json:"order,omitempty"`
	Done     bool   `json:"done,omitempty"`
}

// applyEvents converts a slice of events to concrete tasks.
func applyEvents(events []Event) []Task {
	sort.Slice(events, func(i, j int) bool { return events[i].Time < events[j].Time })

	tasks := make(map[string]*Task)
	for _, ev := range events {
		if ev.EntityType != "task" {
			continue
		}
		switch ev.Type {
		case "task-created":
			var t Task
			if err := json.Unmarshal(ev.Data, &t); err == nil {
				t.ID = ev.EntityID
				tasks[ev.EntityID] = &t
			}
		case "task-updated":
			t, ok := tasks[ev.EntityID]
			if !ok {
				continue
			}
			var changes map[string]interface{}
			if err := json.Unmarshal(ev.Data, &changes); err != nil {
				continue
			}
			if v, ok := changes["title"].(string); ok {
				t.Title = v
			}
			if v, ok := changes["notes"].(string); ok {
				t.Notes = v
			}
			if v, ok := changes["category"].(string); ok {
				t.Category = v
			}
			if v, ok := changes["order"].(float64); ok {
				t.Order = int(v)
			}
		case "task-completed":
			if t, ok := tasks[ev.EntityID]; ok {
				t.Done = true
			}
		}
	}

	result := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, *t)
	}
	return result
}
