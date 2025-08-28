#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

ENV_FILE=tests/docker/env.test
source $ENV_FILE
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

AZ_FUNC_HEALTH_ENDPOINT="/"
tests/docker/wait-for.sh http://localhost:${PRISM_API_PORT}${AZ_FUNC_HEALTH_ENDPOINT} 60
tests/docker/wait-for.sh http://localhost:${STREAM_SERVICE_PORT}${API_HEALTH_ENDPOINT} 60

STREAM_URL=${STREAM_URL:-http://localhost:${STREAM_SERVICE_PORT}/stream} \
SSE_CONNECTIONS=${SSE_CONNECTIONS:-200} \
DURATION_SEC=${DURATION_SEC:-120} \
TEST_BEARER=${TEST_BEARER:-} \
go run tests/perf/sse-load/main.go > tests/perf_sse_summary.txt

