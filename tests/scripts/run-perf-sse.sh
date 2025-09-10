#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

ENV_FILE=tests/docker/env.test
set -a
# shellcheck source=tests/docker/env.test
source "$ENV_FILE"
set +a
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap '$COMPOSE down -v' EXIT

STREAM_SERVICE_BASE=http://localhost:${STREAM_SERVICE_PORT}
PRISM_API_BASE=http://localhost:${PRISM_API_PORT}

tests/docker/wait-for.sh "${PRISM_API_BASE}${AZ_FUNC_HEALTH_ENDPOINT}" 60
tests/docker/wait-for.sh "${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT}" 60

STREAM_URL=${STREAM_URL:-${STREAM_SERVICE_BASE}/stream} \
SSE_CONNECTIONS=${SSE_CONNECTIONS:-200} \
DURATION_SEC=${DURATION_SEC:-120} \
TEST_BEARER=${TEST_BEARER:-$(cd tests/utils && go run ./cmd/gen-token)} \
go run tests/perf/sse-load/main.go > tests/perf_sse_summary.txt

