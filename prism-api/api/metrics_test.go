package api

import (
	"net/http"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestTaskRequestMetricsLogProducesObservabilityEvent(t *testing.T) {
	logger, hook := test.NewNullLogger()
	logger.SetFormatter(&log.JSONFormatter{})

	metrics := newTaskRequestMetrics(logger)
	metrics.start = metrics.start.Add(-50 * time.Millisecond)
	metrics.ObserveAuth(10 * time.Millisecond)
	metrics.ObserveFetch(15 * time.Millisecond)
	metrics.ObserveEncode(5 * time.Millisecond)
	metrics.SetPageTokenProvided(true)
	metrics.SetTasksReturned(3)
	metrics.SetHasNextPage(true)

	metrics.Log(http.StatusOK, nil)

	entry := hook.LastEntry()
	if entry == nil {
		t.Fatal("expected log entry")
	}
	if entry.Message != "observability.event" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}
	if got := entry.Data["event.name"]; got != tasksEventName {
		t.Fatalf("unexpected event name: %v", got)
	}
	if got := entry.Data["event.domain"]; got != tasksEventDomain {
		t.Fatalf("unexpected event domain: %v", got)
	}
	attrsVal, ok := entry.Data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes not logged as map: %#v", entry.Data["attributes"])
	}
	if attrsVal["http.route"] != "/api/tasks" {
		t.Fatalf("unexpected route attribute: %#v", attrsVal["http.route"])
	}
	if attrsVal["prism.tasks.page_token_provided"] != true {
		t.Fatalf("expected page token provided to be true")
	}
	switch v := attrsVal["prism.tasks.tasks_returned"].(type) {
	case float64:
		if v != 3 {
			t.Fatalf("unexpected tasks returned: %v", v)
		}
	case int:
		if v != 3 {
			t.Fatalf("unexpected tasks returned: %v", v)
		}
	default:
		t.Fatalf("unexpected type for tasks returned: %T", v)
	}
	if attrsVal["prism.tasks.has_next_page"] != true {
		t.Fatalf("expected has_next_page true")
	}
	if attrsVal["prism.tasks.total_ms"] == 0.0 {
		t.Fatalf("expected total duration attribute to be set, got %#v", attrsVal["prism.tasks.total_ms"])
	}
	if entry.Data["severity_text"] != "INFO" {
		t.Fatalf("unexpected severity text: %v", entry.Data["severity_text"])
	}
	if entry.Data["severity_number"] != 9 {
		t.Fatalf("unexpected severity number: %v", entry.Data["severity_number"])
	}
}

func TestSeverityForStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		err        error
		wantText   string
		wantNumber int
	}{
		{name: "ok", status: http.StatusOK, wantText: "INFO", wantNumber: 9},
		{name: "warn", status: http.StatusBadRequest, wantText: "WARN", wantNumber: 13},
		{name: "error", status: http.StatusInternalServerError, wantText: "ERROR", wantNumber: 17},
		{name: "errorFromErr", status: 0, err: assertErr{}, wantText: "ERROR", wantNumber: 17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotNumber := severityForStatus(tt.status, tt.err)
			if gotText != tt.wantText || gotNumber != tt.wantNumber {
				t.Fatalf("severityForStatus(%d, %v) = %s/%d, want %s/%d", tt.status, tt.err, gotText, gotNumber, tt.wantText, tt.wantNumber)
			}
		})
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "error" }
