package scenarios

import (
	"net/http"
	"os"
	"testing"
	"time"

	integration "prismtest"
	"prismtest/internal/httpclient"

	"gopkg.in/yaml.v3"
)

type command struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type task struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func newClient(t *testing.T) *httpclient.Client {
	base := os.Getenv("API_BASE")
	if base == "" {
		base = "http://localhost"
	}
	health := os.Getenv("HEALTH_ENDPOINT")
	if health == "" {
		health = "/"
	}
	if _, err := http.Get(base + health); err != nil {
		t.Skipf("skipping, API not reachable: %v", err)
	}
	bearer := os.Getenv("TEST_BEARER")
	if bearer == "" {
		tok, err := integration.TestToken("integration-user")
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		bearer = tok
	}
	return httpclient.New(base, bearer)
}

// pollTasks polls /api/tasks until cond returns true or timeout.
func pollTasks(t *testing.T, client *httpclient.Client, cond func([]task) bool) []task {
	deadline := time.Now().Add(10 * time.Second)
	backoff := 200 * time.Millisecond
	for {
		var tasks []task
		_, err := client.GetJSON("/api/tasks", &tasks)
		if err == nil && cond(tasks) {
			return tasks
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for tasks: %v", err)
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
}

func projectionSLA(t *testing.T) time.Duration {
	sla := 10 * time.Second
	data, err := os.ReadFile("../config.test.yaml")
	if err != nil {
		return sla
	}
	var cfg struct {
		ProjectionSLAMs int `yaml:"projection_visibility_sla_ms"`
	}
	if err := yaml.Unmarshal(data, &cfg); err == nil && cfg.ProjectionSLAMs > 0 {
		sla = time.Duration(cfg.ProjectionSLAMs) * time.Millisecond
	}
	return sla
}
