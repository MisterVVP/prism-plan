package scenarios

import (
	"net/http"
	"os"
	"testing"
	"time"

	integration "prismtest"
	"prismtest/internal/httpclient"
)

type command struct {
	ID         string         `json:"id,omitempty"`
	EntityID   string         `json:"entityId,omitempty"`
	EntityType string         `json:"entityType"`
	Type       string         `json:"type"`
	Data       map[string]any `json:"data,omitempty"`
}

type task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

func getPollTimeout(t *testing.T) time.Duration {
	var err error
	var timeout time.Duration
	envTimeout := os.Getenv("TEST_POLL_TIMEOUT")
	if envTimeout != "" {
		timeout, err = time.ParseDuration(envTimeout)
		if err != nil {
			t.Fatalf("Unable to parse test poll timeout, check TEST_POLL_TIMEOUT variable, error: %v", err)
		}
	} else {
		timeout = 10 * time.Second
	}
	return timeout
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

func newApiClientInner(t *testing.T, baseUrlEnvVarName string, healthEndpointEnvVarName string) *httpclient.Client {
	bearer := getTestBearer(t)
	base := os.Getenv(baseUrlEnvVarName)
	health := os.Getenv(healthEndpointEnvVarName)
	if _, err := http.Get(base + health); err != nil {
		t.Fatalf("API not reachable: %v", err)
	}
	return httpclient.New(base, bearer)
}

func newPrismApiClient(t *testing.T) *httpclient.Client {
	return newApiClientInner(t, "PRISM_API_BASE", "AZ_FUNC_HEALTH_ENDPOINT")
}

func newStreamServiceClient(t *testing.T) *httpclient.Client {
	return newApiClientInner(t, "STREAM_SERVICE_BASE", "API_HEALTH_ENDPOINT")
}

// pollTasks polls /api/tasks until cond returns true or timeout. desc is used to
// identify the condition being waited on so failures are easier to diagnose.
func pollTasks(t *testing.T, client *httpclient.Client, desc string, cond func([]task) bool) []task {
	deadline := time.Now().Add(getPollTimeout(t))
	backoff := 200 * time.Millisecond
	var (
		tasks []task
		err   error
	)
	for {
		tasks = nil
		_, err = client.GetJSON("/api/tasks", &tasks)
		if err == nil && cond(tasks) {
			return tasks
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for tasks for %s: last tasks %v: %v", desc, tasks, err)
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
		}
	}
}
