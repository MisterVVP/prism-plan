# Tests

This workspace contains integration and performance tests for Prism Plan.

## Prerequisites

- Docker and Docker Compose
- Go 1.24+
- [k6](https://k6.io) for API load tests

## Running locally

```bash
bash tests/scripts/run-integration.sh
```

```bash
bash tests/scripts/run-perf-api.sh [path/to/env.file]
```
This script generates a `tests/perf/k6/bearers.json` file containing one bearer token per k6 virtual user. When running locally,
pass `tests/docker/env.test.local` so Docker Compose points to the developer-focused defaults. The script falls back to
`tests/docker/env.test` when no argument is provided (CI configuration).

> **Note:** The API caches only the default task page size. Ensure `PRISM_K6_TASK_PAGE_SIZE` matches the Prism API `TASKS_PAGE_SIZE` (see `tests/docker/env.test`) so the perf run exercises the Redis cache instead of always falling back to Azure Table storage.

```bash
bash tests/scripts/run-perf-sse.sh
```

## Environment variables

- `PRISM_API_LB_BASE` – base URL for API requests (default `http://localhost`)
- `API_HEALTH_ENDPOINT` – endpoint to use as API healthcheck (default `/healthz`)
- `STREAM_URL` – SSE endpoint for streaming tests (default `http://localhost/stream`)
- `TEST_BEARER` – bearer token for authenticated requests in test mode
- `DISABLE_AUTH_FOR_TESTS` – set to `1` to bypass auth if supported
- `ENABLE_DOCKER_CMDS` – allow tests to start/stop services (default `0`)
- `ENABLE_AZURE_ASSERTS` – enable deep Azurite assertions (default `0`)
- `PRISM_K6_ARRIVAL_RATE` – constant arrival rate (iterations per `PRISM_K6_TIME_UNIT`) for k6 open-model scenarios (default `10`)
- `PRISM_K6_PRE_ALLOCATED_VUS` – number of k6 VUs to pre-allocate when using the open model (default `200`)
- `PRISM_K6_MAX_VUS` – maximum number of k6 VUs to allow during the run (default `1000`, capped at `10000`)
- `PRISM_K6_TIME_UNIT` – time window used with `PRISM_K6_ARRIVAL_RATE` (default `1s`)
- `PRISM_K6_DURATION` – total scenario duration (default `30s`)
- `PRISM_K6_TASK_PAGE_SIZE` – overrides the page size used when fetching tasks during perf runs (defaults to `TASKS_PAGE_SIZE` when provided)

Legacy `K6_*` environment variables are deprecated because k6 treats them as global configuration and will override the scripted scenarios. Prefer the `PRISM_K6_*` names.

## Thresholds & SLAs

Projection visibility is expected within 10s by default; tune via `tests/integration/config.test.yaml`.
The k6 scenarios include default thresholds for request failure rate and latency. Adjust them in the scripts or JavaScript files as needed.

