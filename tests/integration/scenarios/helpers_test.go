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
	ID         string                 `json:"id,omitempty"`
	EntityID   string                 `json:"entityId,omitempty"`
	EntityType string                 `json:"entityType"`
	Type       string                 `json:"type"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

type task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

func getEnvVars(baseUrlEnvVarName string, healthEndpointEnvVarName string) (string, string) {
	base := os.Getenv(baseUrlEnvVarName)
	if base == "" {
		base = "http://localhost"
	}
	health := os.Getenv(healthEndpointEnvVarName)
	if health == "" {
		health = "/"
	}
	return base, health
}

func getTestBearer(t *testing.T) string {
	bearer := os.Getenv("TEST_BEARER")
	if bearer == "" {
		tok, err := integration.TestToken("integration-user")
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		bearer = tok
	}
	return bearer
}

func newPrismApiClient(t *testing.T) *httpclient.Client {
	bearer := getTestBearer(t)
	base, health := getEnvVars("PRISM_API_BASE", "AZ_FUNC_HEALTH_ENDPOINT")
	if _, err := http.Get(base + health); err != nil {
		t.Skipf("skipping, API not reachable: %v", err)
	}
	return httpclient.New(base, bearer)
}

func newStreamServiceClient(t *testing.T) *httpclient.Client {
	bearer := getTestBearer(t)
	base, health := getEnvVars("STREAM_SERVICE_BASE", "API_HEALTH_ENDPOINT")
	if _, err := http.Get(base + health); err != nil {
		t.Skipf("skipping, API not reachable: %v", err)
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
