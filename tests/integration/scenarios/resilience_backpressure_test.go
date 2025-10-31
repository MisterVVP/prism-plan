package scenarios

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestResilienceBackpressure(t *testing.T) {
	if os.Getenv("ENABLE_DOCKER_CMDS") != "1" {
		t.Skip("docker control disabled")
	}
	client := newPrismApiClient(t)
	timeout := getPollTimeout(t)

	dc := func(args ...string) error {
		baseArgs := []string{"compose", "-f", "../../../docker-compose.yml", "-f", "../../docker/docker-compose.tests.yml"}
		cmd := exec.Command("docker", append(baseArgs, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := dc("stop", "domain-service-1", "domain-service-2", "domain-service-3", "domain-service-4", "domain-service-5"); err != nil {
		t.Fatalf("stop domain-service: %v", err)
	}
	t.Cleanup(func() {
		dc("start", "domain-service-1", "domain-service-2", "domain-service-3", "domain-service-4", "domain-service-5")
	})

	title := fmt.Sprintf("backpressure-title-%d", time.Now().UnixNano())
	resp, err := client.PostJSON("/api/commands", []command{{IdempotencyKey: fmt.Sprintf("ik-create-%s", title), EntityType: "task", Type: "create-task", Data: map[string]any{"title": title}}}, nil)
	if err != nil {
		t.Fatalf("post command: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	if err := dc("start", "domain-service-1", "domain-service-2", "domain-service-3", "domain-service-4", "domain-service-5"); err != nil {
		t.Fatalf("restart domain-service: %v", err)
	}
	start := time.Now()
	pollTasks(t, client, fmt.Sprintf("task with title %s to appear after restart", title), func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				return true
			}
		}
		return false
	})
	dur := time.Since(start)
	if dur > timeout {
		t.Fatalf("queue drained in %v, exceeds timeout %v", dur, timeout)
	}
}
