package main

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

const (
	tasksEventName   = "prism.api.tasks.request"
	tasksEventDomain = "app"

	attrHTTPStatusCode    = "http.status_code"
	attrTotalMillis       = "prism.tasks.total_ms"
	attrAuthMillis        = "prism.tasks.auth_ms"
	attrFetchMillis       = "prism.tasks.fetch_ms"
	attrEncodeMillis      = "prism.tasks.encode_ms"
	attrTasksReturned     = "prism.tasks.tasks_returned"
	attrPageTokenProvided = "prism.tasks.page_token_provided"
	attrHasNextPage       = "prism.tasks.has_next_page"
	attrErrorStage        = "prism.tasks.error_stage"
)

type logRecord struct {
	EventName      string         `json:"event.name"`
	EventDomain    string         `json:"event.domain"`
	SeverityText   string         `json:"severity_text"`
	SeverityNumber int            `json:"severity_number"`
	Body           any            `json:"body"`
	Attributes     map[string]any `json:"attributes"`
}

type collector struct {
	eventName   string
	eventDomain string
	stats       metricsSummary
	skipped     int
}

type metricsSummary struct {
	Count          int
	SeverityCounts map[string]int
	StatusCounts   map[int]int
	Durations      map[string]*numericStats
	Tasks          *numericStats
	PageTokenTrue  int
	PageTokenFalse int
	HasNextTrue    int
	HasNextFalse   int
	ErrorStages    map[string]int
	ErrorEvents    int
	WarnEvents     int
}

type numericStats struct {
	Count int
	Sum   float64
	Min   float64
	Max   float64
}

type durationSummary struct {
	Count int     `json:"count"`
	Min   float64 `json:"min_ms"`
	Max   float64 `json:"max_ms"`
	Avg   float64 `json:"avg_ms"`
}

type numericSummary struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
}

type boolCounts struct {
	True  int `json:"true"`
	False int `json:"false"`
}

type paginationSummary struct {
	PageToken boolCounts `json:"page_token_provided"`
	HasNext   boolCounts `json:"has_next_page"`
}

type summaryOutput struct {
	EventName      string                     `json:"event_name"`
	EventDomain    string                     `json:"event_domain"`
	TotalEvents    int                        `json:"total_events"`
	SeverityCounts map[string]int             `json:"severity_counts"`
	StatusCounts   map[string]int             `json:"status_counts"`
	DurationMs     map[string]durationSummary `json:"duration_ms"`
	TasksReturned  numericSummary             `json:"tasks_returned"`
	Pagination     paginationSummary          `json:"pagination"`
	ErrorStages    map[string]int             `json:"error_stages,omitempty"`
	ErrorEvents    int                        `json:"error_events"`
	WarnEvents     int                        `json:"warn_events"`
	SkippedLines   int                        `json:"skipped_lines"`
}

func newCollector(eventName, eventDomain string) *collector {
	return &collector{
		eventName:   eventName,
		eventDomain: eventDomain,
		stats: metricsSummary{
			SeverityCounts: make(map[string]int),
			StatusCounts:   make(map[int]int),
			Durations:      make(map[string]*numericStats),
			ErrorStages:    make(map[string]int),
		},
	}
}

func (c *collector) ingest(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	if pipe := strings.Index(trimmed, "|"); pipe >= 0 {
		trimmed = strings.TrimSpace(trimmed[pipe+1:])
	}

	rec, err := decodeRecord(trimmed)
	if err != nil {
		c.skipped++
		return
	}

	if rec.EventName != c.eventName {
		return
	}
	if c.eventDomain != "" && rec.EventDomain != c.eventDomain {
		return
	}

	c.addRecord(rec)
}

func decodeRecord(raw string) (logRecord, error) {
	var rec logRecord
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&rec); err != nil {
		return logRecord{}, err
	}
	return rec, nil
}

func (c *collector) addRecord(rec logRecord) {
	c.stats.Count++

	severity := strings.ToUpper(strings.TrimSpace(rec.SeverityText))
	if severity == "" {
		severity = "UNSPECIFIED"
	}
	c.stats.SeverityCounts[severity]++

	switch severity {
	case "ERROR":
		c.stats.ErrorEvents++
	case "WARN", "WARNING":
		c.stats.WarnEvents++
	}

	if rec.Attributes == nil {
		return
	}

	if raw, exists := rec.Attributes[attrHTTPStatusCode]; exists {
		if status, ok := asInt(raw); ok {
			c.stats.StatusCounts[status]++
		}
	}
	if raw, exists := rec.Attributes[attrTotalMillis]; exists {
		if v, ok := asFloat(raw); ok {
			c.stats.addDuration("total", v)
		}
	}
	if raw, exists := rec.Attributes[attrAuthMillis]; exists {
		if v, ok := asFloat(raw); ok {
			c.stats.addDuration("auth", v)
		}
	}
	if raw, exists := rec.Attributes[attrFetchMillis]; exists {
		if v, ok := asFloat(raw); ok {
			c.stats.addDuration("fetch", v)
		}
	}
	if raw, exists := rec.Attributes[attrEncodeMillis]; exists {
		if v, ok := asFloat(raw); ok {
			c.stats.addDuration("encode", v)
		}
	}
	if raw, exists := rec.Attributes[attrTasksReturned]; exists {
		if v, ok := asFloat(raw); ok {
			if c.stats.Tasks == nil {
				c.stats.Tasks = newNumericStats()
			}
			c.stats.Tasks.add(v)
		}
	}
	if raw, exists := rec.Attributes[attrPageTokenProvided]; exists {
		if b, ok := asBool(raw); ok {
			if b {
				c.stats.PageTokenTrue++
			} else {
				c.stats.PageTokenFalse++
			}
		}
	}
	if raw, exists := rec.Attributes[attrHasNextPage]; exists {
		if b, ok := asBool(raw); ok {
			if b {
				c.stats.HasNextTrue++
			} else {
				c.stats.HasNextFalse++
			}
		}
	}
	if raw, exists := rec.Attributes[attrErrorStage]; exists {
		if stage, ok := asString(raw); ok && stage != "" {
			c.stats.ErrorStages[stage]++
		}
	}
}

func (s *metricsSummary) addDuration(key string, value float64) {
	stat, ok := s.Durations[key]
	if !ok {
		stat = newNumericStats()
		s.Durations[key] = stat
	}
	stat.add(value)
}

func newNumericStats() *numericStats {
	return &numericStats{Min: math.MaxFloat64}
}

func (n *numericStats) add(value float64) {
	n.Count++
	n.Sum += value
	if value < n.Min {
		n.Min = value
	}
	if value > n.Max {
		n.Max = value
	}
}

func (n *numericStats) toDurationSummary() durationSummary {
	if n == nil || n.Count == 0 {
		return durationSummary{}
	}
	min := n.Min
	if n.Min == math.MaxFloat64 {
		min = 0
	}
	return durationSummary{
		Count: n.Count,
		Min:   min,
		Max:   n.Max,
		Avg:   n.Sum / float64(n.Count),
	}
}

func (n *numericStats) toNumericSummary() numericSummary {
	if n == nil || n.Count == 0 {
		return numericSummary{}
	}
	min := n.Min
	if n.Min == math.MaxFloat64 {
		min = 0
	}
	return numericSummary{
		Count: n.Count,
		Min:   min,
		Max:   n.Max,
		Avg:   n.Sum / float64(n.Count),
	}
}

func (c *collector) summary() summaryOutput {
	durationMap := make(map[string]durationSummary, len(c.stats.Durations))
	for key, stat := range c.stats.Durations {
		durationMap[key] = stat.toDurationSummary()
	}

	statusCounts := make(map[string]int, len(c.stats.StatusCounts))
	for status, count := range c.stats.StatusCounts {
		statusCounts[strconv.Itoa(status)] = count
	}

	severity := make(map[string]int, len(c.stats.SeverityCounts))
	for k, v := range c.stats.SeverityCounts {
		severity[k] = v
	}

	return summaryOutput{
		EventName:      c.eventName,
		EventDomain:    c.eventDomain,
		TotalEvents:    c.stats.Count,
		SeverityCounts: severity,
		StatusCounts:   statusCounts,
		DurationMs:     durationMap,
		TasksReturned:  c.stats.Tasks.toNumericSummary(),
		Pagination: paginationSummary{
			PageToken: boolCounts{True: c.stats.PageTokenTrue, False: c.stats.PageTokenFalse},
			HasNext:   boolCounts{True: c.stats.HasNextTrue, False: c.stats.HasNextFalse},
		},
		ErrorStages:  compactStringIntMap(c.stats.ErrorStages),
		ErrorEvents:  c.stats.ErrorEvents,
		WarnEvents:   c.stats.WarnEvents,
		SkippedLines: c.skipped,
	}
}

func compactStringIntMap(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (s summaryOutput) ShortString() string {
	total := s.TotalEvents
	info := s.SeverityCounts["INFO"] + s.SeverityCounts["INFORMATION"]
	warn := s.WarnEvents
	errCount := s.ErrorEvents
	totalSummary, ok := s.DurationMs["total"]
	var totalDur, maxDur float64
	if ok {
		totalDur = totalSummary.Avg
		maxDur = totalSummary.Max
	}
	return strings.TrimSpace(strings.Join([]string{
		"event=" + s.EventName,
		"domain=" + s.EventDomain,
		"total=" + strconv.Itoa(total),
		"info=" + strconv.Itoa(info),
		"warn=" + strconv.Itoa(warn),
		"error=" + strconv.Itoa(errCount),
		"avg_total_ms=" + formatFloat(totalDur),
		"max_total_ms=" + formatFloat(maxDur),
	}, " "))
}

func formatFloat(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func asFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func asInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func asBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(v) {
		case "true":
			return true, true
		case "false":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func asString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	default:
		return "", false
	}
}
