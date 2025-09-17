package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskMarshalIncludesZeroOrder(t *testing.T) {
	task := Task{ID: "t1", Title: "Title", Category: "normal", Order: 0}

	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}

	if !strings.Contains(string(payload), "\"order\":0") {
		t.Fatalf("expected order field to be present, got %s", payload)
	}
}
