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
bash tests/scripts/run-perf-api.sh
```
This script generates a `tests/perf/k6/bearers.json` file containing one bearer token per k6 virtual user.

```bash
bash tests/scripts/run-perf-sse.sh
```

## Environment variables

- `PRISM_API_BASE` – base URL for API requests (default `http://localhost`)
- `AZ_FUNC_HEALTH_ENDPOINT` – endpoint to use as healthcheck (default `/`)
- `STREAM_URL` – SSE endpoint for streaming tests (default `http://localhost/stream`)
- `TEST_BEARER` – bearer token for authenticated requests in test mode
- `DISABLE_AUTH_FOR_TESTS` – set to `1` to bypass auth if supported
- `ENABLE_DOCKER_CMDS` – allow tests to start/stop services (default `0`)
- `ENABLE_AZURE_ASSERTS` – enable deep Azurite assertions (default `0`)

## Thresholds & SLAs

Projection visibility is expected within 10s by default; tune via `tests/integration/config.test.yaml`.
The k6 scenarios include default thresholds for request failure rate and latency. Adjust them in the scripts or JavaScript files as needed.

