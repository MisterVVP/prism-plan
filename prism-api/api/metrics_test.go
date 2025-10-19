package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTaskRequestMetricsLogProducesObservabilityEvent(t *testing.T) {
	logger, hook := test.NewNullLogger()
	logger.SetFormatter(&log.JSONFormatter{})

	tp, exporter, restore := setupTestTracer(t)
	defer restore()

	metrics, _ := newTaskRequestMetrics(context.Background(), logger)
	metrics.start = metrics.start.Add(-50 * time.Millisecond)
	metrics.ObserveAuth(10 * time.Millisecond)
	metrics.ObserveFetch(15 * time.Millisecond)
	metrics.ObserveEncode(5 * time.Millisecond)
	metrics.SetPageTokenProvided(true)
	metrics.SetTasksReturned(3)
	metrics.SetHasNextPage(true)

	metrics.Log(http.StatusOK, nil)

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush spans: %v", err)
	}

	entry := waitForLogEntry(t, hook, time.Second)
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
	if traceID, ok := entry.Data["trace_id"].(string); !ok || traceID == "" {
		t.Fatalf("expected trace_id to be recorded, got %#v", entry.Data["trace_id"])
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name != tasksSpanName {
		t.Fatalf("unexpected span name: %s", span.Name)
	}
	spanAttrs := attributesToMap(span.Attributes)
	if spanAttrs["http.route"] != "/api/tasks" {
		t.Fatalf("span route attribute mismatch: %#v", spanAttrs["http.route"])
	}
	if code, ok := spanAttrs["http.status_code"].(int64); !ok || code != int64(http.StatusOK) {
		t.Fatalf("unexpected http.status_code on span: %#v", spanAttrs["http.status_code"])
	}
	if stage, exists := spanAttrs["prism.tasks.error_stage"]; exists && stage != "" {
		t.Fatalf("expected no error stage, got %#v", stage)
	}
	if span.Status.Code != codes.Ok {
		t.Fatalf("expected span status Ok, got %v", span.Status.Code)
	}

	var event sdktrace.Event
	for _, ev := range span.Events {
		if ev.Name == "observability.event" {
			event = ev
			break
		}
	}
	if event.Name == "" {
		t.Fatalf("expected observability.event span event, got %#v", span.Events)
	}
	eventAttrs := attributesToMap(event.Attributes)
	if eventAttrs["event.name"] != tasksEventName {
		t.Fatalf("unexpected event.name attribute: %#v", eventAttrs["event.name"])
	}
	if eventAttrs["event.domain"] != tasksEventDomain {
		t.Fatalf("unexpected event.domain attribute: %#v", eventAttrs["event.domain"])
	}
	if eventAttrs["severity_text"] != "INFO" {
		t.Fatalf("unexpected span event severity: %#v", eventAttrs["severity_text"])
	}
	if total, ok := eventAttrs["prism.tasks.total_ms"].(float64); !ok || total == 0 {
		t.Fatalf("expected span event total_ms to be set, got %#v", eventAttrs["prism.tasks.total_ms"])
	}
}

func TestTaskRequestMetricsLogWithErrorSetsSpanStatus(t *testing.T) {
	logger, _ := test.NewNullLogger()
	logger.SetFormatter(&log.JSONFormatter{})

	tp, exporter, restore := setupTestTracer(t)
	defer restore()

	metrics, _ := newTaskRequestMetrics(context.Background(), logger)
	metrics.SetErrorStage("storage")
	boom := errors.New("storage failure")

	metrics.Log(http.StatusInternalServerError, boom)

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush spans: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Fatalf("expected span status error, got %v", span.Status.Code)
	}
	if span.Status.Description == "" {
		t.Fatalf("expected status description for error")
	}

	var obsEvent sdktrace.Event
	for _, ev := range span.Events {
		if ev.Name == "observability.event" {
			obsEvent = ev
			break
		}
	}
	if obsEvent.Name == "" {
		t.Fatalf("expected observability event in span events, got %#v", span.Events)
	}
	attrs := attributesToMap(obsEvent.Attributes)
	if attrs["severity_text"] != "ERROR" {
		t.Fatalf("unexpected severity_text for error: %#v", attrs["severity_text"])
	}
	if attrs["prism.tasks.error_stage"] != "storage" {
		t.Fatalf("expected error stage attribute propagated, got %#v", attrs["prism.tasks.error_stage"])
	}
	if attrs["error.message"] != boom.Error() {
		t.Fatalf("expected error.message attribute, got %#v", attrs["error.message"])
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

func setupTestTracer(t *testing.T) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter, func()) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	cleanup := func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Logf("shutdown tracer provider: %v", err)
		}
		otel.SetTracerProvider(prev)
	}
	return tp, exporter, cleanup
}

func attributesToMap(attrs []attribute.KeyValue) map[string]any {
	out := make(map[string]any, len(attrs))
	for _, kv := range attrs {
		out[string(kv.Key)] = kv.Value.AsInterface()
	}
	return out
}

func waitForLogEntry(t *testing.T, hook *test.Hook, timeout time.Duration) *log.Entry {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if entry := hook.LastEntry(); entry != nil {
			return entry
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected log entry within %v", timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
