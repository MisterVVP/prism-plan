package api

import (
	"context"
	"net/http"
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

	totalMillis := durationToMillis(time.Since(m.start))
	attributes := map[string]any{
		"http.route":                      "/api/tasks",
		"http.status_code":                status,
		"prism.tasks.total_ms":            totalMillis,
		"prism.tasks.page_token_provided": m.pageTokenProvided,
		"prism.tasks.tasks_returned":      m.tasksReturned,
		"prism.tasks.has_next_page":       m.hasNextPage,
		"prism.tasks.request_start_ns":    m.start.UnixNano(),
	}

	if m.authDuration > 0 {
		attributes["prism.tasks.auth_ms"] = durationToMillis(m.authDuration)
	}
	if m.fetchDuration > 0 {
		attributes["prism.tasks.fetch_ms"] = durationToMillis(m.fetchDuration)
	}
	if m.encodeDuration > 0 {
		attributes["prism.tasks.encode_ms"] = durationToMillis(m.encodeDuration)
	}
	if m.errorStage != "" {
		attributes["prism.tasks.error_stage"] = m.errorStage
	}
	if err != nil {
		attributes["error.message"] = err.Error()
	}

	eventTime := time.Now()
	severityText, severityNumber := severityForStatus(status, err)

	if m.span != nil {
		spanContext := m.span.SpanContext()
		kv := attributesToKeyValues(attributes)
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

	fields := log.Fields{
		"time_unix_nano":          eventTime.UnixNano(),
		"observed_time_unix_nano": eventTime.UnixNano(),
		"severity_text":           severityText,
		"severity_number":         severityNumber,
		"body":                    tasksEventBody,
		"event.name":              tasksEventName,
		"event.domain":            tasksEventDomain,
		"attributes":              attributes,
	}

	if m.spanContext.HasTraceID() {
		fields["trace_id"] = m.spanContext.TraceID().String()
	}
	if m.spanContext.HasSpanID() {
		fields["span_id"] = m.spanContext.SpanID().String()
	}

	m.logger.WithFields(fields).Info("observability.event")
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

func attributesToKeyValues(attrs map[string]any) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}

	out := make([]attribute.KeyValue, 0, len(attrs))
	for key, val := range attrs {
		switch v := val.(type) {
		case string:
			out = append(out, attribute.String(key, v))
		case bool:
			out = append(out, attribute.Bool(key, v))
		case int:
			out = append(out, attribute.Int(key, v))
		case int64:
			out = append(out, attribute.Int64(key, v))
		case uint:
			out = append(out, attribute.Int(key, int(v)))
		case uint64:
			out = append(out, attribute.Int64(key, int64(v)))
		case float64:
			out = append(out, attribute.Float64(key, v))
		case float32:
			out = append(out, attribute.Float64(key, float64(v)))
		default:
			// skip unsupported types to avoid panics when exporting spans
		}
	}
	return out
}
