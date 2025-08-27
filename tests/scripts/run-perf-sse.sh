#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

ENV_FILE=tests/docker/env.test.example
PRISM_API_PORT=$(grep PRISM_API_PORT "$ENV_FILE" | cut -d'=' -f2)
STREAM_SERVICE_PORT=$(grep STREAM_SERVICE_PORT "$ENV_FILE" | cut -d'=' -f2)
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

HEALTH_ENDPOINT="/"
tests/docker/wait-for.sh http://localhost:${PRISM_API_PORT}${HEALTH_ENDPOINT} 60

STREAM_URL=${STREAM_URL:-http://localhost:${STREAM_SERVICE_PORT}/stream} \
SSE_CONNECTIONS=${SSE_CONNECTIONS:-200} \
DURATION_SEC=${DURATION_SEC:-120} \
TEST_BEARER=${TEST_BEARER:-} \
go run tests/perf/sse-load/main.go > tests/perf_sse_summary.txt

