package api

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tasksSpanName    = "GET /api/tasks"
	tasksEventName   = "prism.api.tasks.request"
	tasksEventDomain = "app"
	tasksEventBody   = "tasks request completed"
)

type taskRequestMetrics struct {
	logger            *log.Logger
	span              trace.Span
	spanContext       trace.SpanContext
	start             time.Time
	authDuration      time.Duration
	fetchDuration     time.Duration
	encodeDuration    time.Duration
	pageTokenProvided bool
	tasksReturned     int
	hasNextPage       bool
	errorStage        string
}

type taskLogRecord struct {
	status            int
	total             time.Duration
	auth              time.Duration
	fetch             time.Duration
	encode            time.Duration
	requestStartNS    int64
	tasksReturned     int
	pageTokenProvided bool
	hasNextPage       bool
	errorStage        string
	errorMessage      string
}

type taskLogEvent struct {
	logger         *log.Logger
	record         taskLogRecord
	eventTime      time.Time
	severityText   string
	severityNumber int
	traceID        string
	spanID         string
}

var (
	taskLogQueueOnce sync.Once
	taskLogQueue     chan taskLogEvent
)

func newTaskRequestMetrics(ctx context.Context, logger *log.Logger) (*taskRequestMetrics, context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	tracer := otel.Tracer("prism-api/api/tasks")
	spanCtx, span := tracer.Start(ctx, tasksSpanName, trace.WithSpanKind(trace.SpanKindServer), trace.WithTimestamp(start))
	span.SetAttributes(
		attribute.String("http.route", "/api/tasks"),
		attribute.String("http.method", http.MethodGet),
	)

	return &taskRequestMetrics{
		logger:      logger,
		span:        span,
		spanContext: span.SpanContext(),
		start:       start,
	}, spanCtx
}

func (m *taskRequestMetrics) ObserveAuth(duration time.Duration) {
	if duration <= 0 {
		return
	}
	m.authDuration = duration
}

func (m *taskRequestMetrics) ObserveFetch(duration time.Duration) {
	if duration <= 0 {
		return
	}
	m.fetchDuration = duration
}

func (m *taskRequestMetrics) ObserveEncode(duration time.Duration) {
	if duration <= 0 {
		return
	}
	m.encodeDuration = duration
}

func (m *taskRequestMetrics) SetPageTokenProvided(provided bool) {
	m.pageTokenProvided = provided
}

func (m *taskRequestMetrics) SetTasksReturned(count int) {
	if count < 0 {
		count = 0
	}
	m.tasksReturned = count
}

func (m *taskRequestMetrics) SetHasNextPage(hasNext bool) {
	m.hasNextPage = hasNext
}

func (m *taskRequestMetrics) SetErrorStage(stage string) {
	if stage == "" {
		return
	}
	m.errorStage = stage
}

func (m *taskRequestMetrics) Log(status int, err error) {
	if m == nil {
		return
	}

	record := taskLogRecord{
		status:            status,
		total:             time.Since(m.start),
		auth:              m.authDuration,
		fetch:             m.fetchDuration,
		encode:            m.encodeDuration,
		requestStartNS:    m.start.UnixNano(),
		tasksReturned:     m.tasksReturned,
		pageTokenProvided: m.pageTokenProvided,
		hasNextPage:       m.hasNextPage,
		errorStage:        m.errorStage,
	}
	if err != nil {
		record.errorMessage = err.Error()
	}

	eventTime := time.Now()
	severityText, severityNumber := severityForStatus(status, err)

	if m.span != nil {
		spanContext := m.span.SpanContext()
		kv := record.keyValues()
		eventKV := append(kv,
			attribute.String("event.name", tasksEventName),
			attribute.String("event.domain", tasksEventDomain),
			attribute.String("body", tasksEventBody),
			attribute.String("severity_text", severityText),
			attribute.Int("severity_number", severityNumber),
		)

		if err != nil {
			m.span.RecordError(err)
		}

		if severityNumber >= 17 || status >= http.StatusBadRequest || err != nil {
			msg := http.StatusText(status)
			if msg == "" && err != nil {
				msg = err.Error()
			}
			m.span.SetStatus(codes.Error, msg)
		} else {
			m.span.SetStatus(codes.Ok, "")
		}

		m.span.SetAttributes(kv...)
		m.span.AddEvent("observability.event", trace.WithTimestamp(eventTime), trace.WithAttributes(eventKV...))
		m.span.End(trace.WithTimestamp(eventTime))
		m.spanContext = spanContext
		m.span = nil
	}

	if m.logger == nil {
		return
	}

	event := taskLogEvent{
		logger:         m.logger,
		record:         record,
		eventTime:      eventTime,
		severityText:   severityText,
		severityNumber: severityNumber,
	}

	if m.spanContext.HasTraceID() {
		event.traceID = m.spanContext.TraceID().String()
	}
	if m.spanContext.HasSpanID() {
		event.spanID = m.spanContext.SpanID().String()
	}

	if !enqueueTaskLogEvent(event) {
		event.log()
	}
}

func durationToMillis(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(d) / float64(time.Millisecond)
}

func severityForStatus(status int, err error) (string, int) {
	if err != nil && status == 0 {
		status = http.StatusInternalServerError
	}

	switch {
	case status >= http.StatusInternalServerError || err != nil:
		return "ERROR", 17
	case status >= http.StatusBadRequest:
		return "WARN", 13
	default:
		return "INFO", 9
	}
}

func (r *taskLogRecord) keyValues() []attribute.KeyValue {
	kv := []attribute.KeyValue{
		attribute.String("http.route", "/api/tasks"),
		attribute.Int("http.status_code", r.status),
		attribute.Float64("prism.tasks.total_ms", durationToMillis(r.total)),
		attribute.Bool("prism.tasks.page_token_provided", r.pageTokenProvided),
		attribute.Int("prism.tasks.tasks_returned", r.tasksReturned),
		attribute.Bool("prism.tasks.has_next_page", r.hasNextPage),
		attribute.Int64("prism.tasks.request_start_ns", r.requestStartNS),
	}

	if r.auth > 0 {
		kv = append(kv, attribute.Float64("prism.tasks.auth_ms", durationToMillis(r.auth)))
	}
	if r.fetch > 0 {
		kv = append(kv, attribute.Float64("prism.tasks.fetch_ms", durationToMillis(r.fetch)))
	}
	if r.encode > 0 {
		kv = append(kv, attribute.Float64("prism.tasks.encode_ms", durationToMillis(r.encode)))
	}
	if r.errorStage != "" {
		kv = append(kv, attribute.String("prism.tasks.error_stage", r.errorStage))
	}
	if r.errorMessage != "" {
		kv = append(kv, attribute.String("error.message", r.errorMessage))
	}
	return kv
}

func (r *taskLogRecord) attributesMap() map[string]any {
	attrs := map[string]any{
		"http.route":                      "/api/tasks",
		"http.status_code":                r.status,
		"prism.tasks.total_ms":            durationToMillis(r.total),
		"prism.tasks.page_token_provided": r.pageTokenProvided,
		"prism.tasks.tasks_returned":      r.tasksReturned,
		"prism.tasks.has_next_page":       r.hasNextPage,
		"prism.tasks.request_start_ns":    r.requestStartNS,
	}
	if r.auth > 0 {
		attrs["prism.tasks.auth_ms"] = durationToMillis(r.auth)
	}
	if r.fetch > 0 {
		attrs["prism.tasks.fetch_ms"] = durationToMillis(r.fetch)
	}
	if r.encode > 0 {
		attrs["prism.tasks.encode_ms"] = durationToMillis(r.encode)
	}
	if r.errorStage != "" {
		attrs["prism.tasks.error_stage"] = r.errorStage
	}
	if r.errorMessage != "" {
		attrs["error.message"] = r.errorMessage
	}
	return attrs
}

func (e *taskLogEvent) log() {
	if e.logger == nil {
		return
	}
	e.logger.WithFields(e.fields()).Info("observability.event")
}

func (e *taskLogEvent) fields() log.Fields {
	fields := log.Fields{
		"time_unix_nano":          e.eventTime.UnixNano(),
		"observed_time_unix_nano": e.eventTime.UnixNano(),
		"severity_text":           e.severityText,
		"severity_number":         e.severityNumber,
		"body":                    tasksEventBody,
		"event.name":              tasksEventName,
		"event.domain":            tasksEventDomain,
		"attributes":              e.record.attributesMap(),
	}

	if e.traceID != "" {
		fields["trace_id"] = e.traceID
	}
	if e.spanID != "" {
		fields["span_id"] = e.spanID
	}

	return fields
}

func enqueueTaskLogEvent(event taskLogEvent) bool {
	if event.logger == nil {
		return true
	}

	taskLogQueueOnce.Do(initTaskLogQueue)

	select {
	case taskLogQueue <- event:
		return true
	default:
		return false
	}
}

func initTaskLogQueue() {
	workers := envInt("TASK_METRICS_LOG_WORKERS", runtime.NumCPU())
	if workers < 1 {
		workers = 1
	}
	buffer := envInt("TASK_METRICS_LOG_QUEUE", workers*1024)
	if buffer < 1 {
		buffer = workers * 1024
	}

	taskLogQueue = make(chan taskLogEvent, buffer)
	for i := 0; i < workers; i++ {
		go func() {
			for event := range taskLogQueue {
				event.log()
			}
		}()
	}
}
