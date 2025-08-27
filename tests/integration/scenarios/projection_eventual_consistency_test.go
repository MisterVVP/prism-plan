package scenarios

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestProjectionEventualConsistency(t *testing.T) {
	client := newClient(t)
	sla := projectionSLA(t)

	title := fmt.Sprintf("consistency-%d", time.Now().UnixNano())
	start := time.Now()
	resp, err := client.PostJSON("/api/commands", command{Type: "CreateTask", Payload: map[string]interface{}{"title": title}}, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				return true
			}
		}
		return false
	})
	dur := time.Since(start)
	if dur > sla {
		t.Fatalf("projection took %v, exceeds SLA %v", dur, sla)
	}
}
