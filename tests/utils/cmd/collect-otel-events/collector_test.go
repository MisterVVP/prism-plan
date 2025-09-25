package main

import "testing"

func TestCollectorAggregatesOtelEvents(t *testing.T) {
	collector := newCollector(tasksEventName, tasksEventDomain)

	lines := []string{
		`{"event.name":"prism.api.tasks.request","event.domain":"app","severity_text":"INFO","severity_number":9,"attributes":{"http.status_code":200,"prism.tasks.total_ms":40.5,"prism.tasks.auth_ms":5.0,"prism.tasks.fetch_ms":10.2,"prism.tasks.encode_ms":3.3,"prism.tasks.tasks_returned":12,"prism.tasks.page_token_provided":true,"prism.tasks.has_next_page":false}}`,
		`non-json line`,
		`{"event.name":"prism.api.tasks.request","event.domain":"app","severity_text":"WARN","severity_number":13,"attributes":{"http.status_code":429,"prism.tasks.total_ms":60.0,"prism.tasks.tasks_returned":8,"prism.tasks.page_token_provided":false,"prism.tasks.has_next_page":true,"prism.tasks.error_stage":"storage"}}`,
	}

	for _, line := range lines {
		collector.ingest(line)
	}

	summary := collector.summary()

	if summary.TotalEvents != 2 {
		t.Fatalf("expected 2 events, got %d", summary.TotalEvents)
	}
	if summary.SeverityCounts["INFO"] != 1 {
		t.Fatalf("expected 1 info event, got %d", summary.SeverityCounts["INFO"])
	}
	if summary.WarnEvents != 1 {
		t.Fatalf("expected 1 warn event, got %d", summary.WarnEvents)
	}
	if summary.StatusCounts["200"] != 1 || summary.StatusCounts["429"] != 1 {
		t.Fatalf("unexpected status counts: %#v", summary.StatusCounts)
	}

	totalStats, ok := summary.DurationMs["total"]
	if !ok || totalStats.Count != 2 {
		t.Fatalf("expected total duration stats for 2 events, got %#v", totalStats)
	}
	if totalStats.Avg <= 0 {
		t.Fatalf("expected avg duration >0, got %f", totalStats.Avg)
	}

	if summary.TasksReturned.Count != 2 {
		t.Fatalf("expected tasks returned count 2, got %d", summary.TasksReturned.Count)
	}
	if summary.Pagination.PageToken.True != 1 || summary.Pagination.PageToken.False != 1 {
		t.Fatalf("unexpected page token counts: %#v", summary.Pagination.PageToken)
	}
	if summary.Pagination.HasNext.True != 1 || summary.Pagination.HasNext.False != 1 {
		t.Fatalf("unexpected has next counts: %#v", summary.Pagination.HasNext)
	}
	if summary.ErrorStages["storage"] != 1 {
		t.Fatalf("expected storage error stage, got %#v", summary.ErrorStages)
	}

	if summary.ShortString() == "" {
		t.Fatal("expected short summary to be non-empty")
	}
}
