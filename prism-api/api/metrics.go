package api

import (
	"time"

	log "github.com/sirupsen/logrus"
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

	fields := log.Fields{
		"route":               "/api/tasks",
		"status":              status,
		"total_ms":            durationToMillis(time.Since(m.start)),
		"page_token_provided": m.pageTokenProvided,
		"tasks_returned":      m.tasksReturned,
		"has_next_page":       m.hasNextPage,
	}

	if m.authDuration > 0 {
		fields["auth_ms"] = durationToMillis(m.authDuration)
	}
	if m.fetchDuration > 0 {
		fields["fetch_ms"] = durationToMillis(m.fetchDuration)
	}
	if m.encodeDuration > 0 {
		fields["encode_ms"] = durationToMillis(m.encodeDuration)
	}
	if m.errorStage != "" {
		fields["error_stage"] = m.errorStage
	}
	if err != nil {
		fields["error"] = err.Error()
	}

	m.logger.WithFields(fields).Info("tasks.request.metrics")
}

func durationToMillis(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(d) / float64(time.Millisecond)
}
