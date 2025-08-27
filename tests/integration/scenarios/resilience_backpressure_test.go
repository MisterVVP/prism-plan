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
	client := newClient(t)
	sla := projectionSLA(t)

	dc := func(args ...string) error {
		baseArgs := []string{"compose", "-f", "../../docker-compose.yml", "-f", "../docker/docker-compose.tests.yml"}
		cmd := exec.Command("docker", append(baseArgs, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := dc("stop", "domain-service"); err != nil {
		t.Fatalf("stop domain-service: %v", err)
	}
	t.Cleanup(func() { dc("start", "domain-service") })

	title := fmt.Sprintf("backpressure-%d", time.Now().UnixNano())
	resp, err := client.PostJSON("/api/commands", command{Type: "CreateTask", Payload: map[string]interface{}{"title": title}}, nil)
	if err != nil {
		t.Fatalf("post command: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	if err := dc("start", "domain-service"); err != nil {
		t.Fatalf("restart domain-service: %v", err)
	}
	start := time.Now()
	pollTasks(t, client, func(ts []task) bool {
		for _, tk := range ts {
			if tk.Title == title {
				return true
			}
		}
		return false
	})
	dur := time.Since(start)
	if dur > sla {
		t.Fatalf("queue drained in %v, exceeds SLA %v", dur, sla)
	}
}
