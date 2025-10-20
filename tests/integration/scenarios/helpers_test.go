package scenarios

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"prismtest/internal/httpclient"
	testutil "prismtestutil"
)

type command struct {
	IdempotencyKey string         `json:"idempotencyKey,omitempty"`
	EntityType     string         `json:"entityType"`
	Type           string         `json:"type"`
	Data           map[string]any `json:"data,omitempty"`
}

type task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Notes    string `json:"notes"`
	Done     bool   `json:"done"`
	Category string `json:"category"`
	Order    int    `json:"order"`
}

type taskPage struct {
	Tasks         []task `json:"tasks"`
	NextPageToken string `json:"nextPageToken"`
}

func fetchAllTasks(t *testing.T, client *httpclient.Client) ([]task, error) {
	t.Helper()
	var (
		tasks []task
		page  taskPage
		token string
	)
	for {
		path := "/api/tasks"
		if token != "" {
			path += "?pageToken=" + url.QueryEscape(token)
		}
		resp, err := client.GetJSON(path, &page)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}
		tasks = append(tasks, page.Tasks...)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	return tasks, nil
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
		tok, err := testutil.TestToken("integration-user")
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
		t.Fatalf("API not reachable at %s, error: %v", base+health, err)
	}
	return httpclient.New(base, bearer)
}

func newPrismApiClient(t *testing.T) *httpclient.Client {
	return newApiClientInner(t, "PRISM_API_LB_BASE", "API_HEALTH_ENDPOINT")
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
		tasks, err = fetchAllTasks(t, client)
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
