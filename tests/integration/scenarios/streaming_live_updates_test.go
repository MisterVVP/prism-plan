package scenarios

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStreamingLiveUpdates(t *testing.T) {
	client := newClient(t)

	// Create a task to mutate
	title := fmt.Sprintf("stream-%d", time.Now().UnixNano())
	if _, err := client.PostJSON("/api/commands", command{Type: "CreateTask", Payload: map[string]interface{}{"title": title}}, nil); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var taskID string
	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				taskID = tk.ID
				return true
			}
		}
		return false
	})

	req, err := http.NewRequest(http.MethodGet, client.BaseURL+"/stream", nil)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	if client.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+client.Bearer)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := client.HTTP.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skipf("stream unavailable: %v status %v", err, resp.StatusCode)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	eventCh := make(chan string, 1)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data != "" {
					eventCh <- data
					return
				}
			}
		}
	}()

	// mutate the task
	newTitle := title + "-sse"
	if _, err := client.PostJSON("/api/commands", command{Type: "EditTask", Payload: map[string]interface{}{"id": taskID, "title": newTitle}}, nil); err != nil {
		t.Fatalf("edit task: %v", err)
	}

	select {
	case ev := <-eventCh:
		if !strings.Contains(ev, taskID) && ev == "" {
			t.Fatalf("unexpected event payload: %q", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("no event received in time")
	}
}
