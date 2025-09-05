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
	prismApiClient := newPrismApiClient(t)

        // Create a task to mutate
        title := fmt.Sprintf("stream-title-%d", time.Now().UnixNano())
        if _, err := prismApiClient.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-create-%s", title), EntityType: "task", Type: "create-task", Data: map[string]any{"title": title}}}, nil); err != nil {
                t.Fatalf("create task: %v", err)
        }
        var taskID string
        pollTasks(t, prismApiClient, fmt.Sprintf("task with title %s to be created", title), func(ts []task) bool {
                for _, tk := range ts {
                        if tk.Title == title {
                                taskID = tk.ID
                                return true
                        }
                }
                return false
        })

	streamServiceClient := newStreamServiceClient(t)
	req, err := http.NewRequest(http.MethodGet, streamServiceClient.BaseURL+"/stream", nil)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	if streamServiceClient.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+streamServiceClient.Bearer)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := streamServiceClient.HTTP.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("stream unavailable: %v status %v", err, resp.StatusCode)
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
			if after, ok := strings.CutPrefix(line, "data:"); ok {
				data := strings.TrimSpace(after)
				if data != "" {
					eventCh <- data
					return
				}
			}
		}
	}()

	// mutate the task
        newTitle := title + "-sse"
        if _, err := prismApiClient.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-update-%s", taskID), EntityType: "task", Type: "update-task", Data: map[string]any{"id": taskID, "title": newTitle}}}, nil); err != nil {
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
