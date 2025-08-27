#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

ENV_FILE=tests/docker/env.test.example
FRONTEND_PORT=$(grep FRONTEND_PORT "$ENV_FILE" | cut -d'=' -f2)
STREAM_SERVICE_PORT=$(grep STREAM_SERVICE_PORT "$ENV_FILE" | cut -d'=' -f2)
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

tests/docker/wait-for.sh https://localhost:${FRONTEND_PORT}/healthz

STREAM_URL=${STREAM_URL:-http://localhost:${STREAM_SERVICE_PORT}/stream} \
SSE_CONNECTIONS=${SSE_CONNECTIONS:-200} \
DURATION_SEC=${DURATION_SEC:-120} \
TEST_BEARER=${TEST_BEARER:-} \
go run tests/perf/sse-load/main.go > tests/perf_sse_summary.txt

