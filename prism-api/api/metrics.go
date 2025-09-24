package api

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	tasksEventName   = "prism.api.tasks.request"
	tasksEventDomain = "app"
	tasksEventBody   = "tasks request completed"
)

type taskRequestMetrics struct {
	logger            *log.Logger
	start             time.Time
	authDuration      time.Duration
	fetchDuration     time.Duration
	encodeDuration    time.Duration
	pageTokenProvided bool
	tasksReturned     int
	hasNextPage       bool
	errorStage        string
}

func newTaskRequestMetrics(logger *log.Logger) *taskRequestMetrics {
	return &taskRequestMetrics{
		logger: logger,
		start:  time.Now(),
	}
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
	if m == nil || m.logger == nil {
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
