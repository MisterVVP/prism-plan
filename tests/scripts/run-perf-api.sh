#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

ENV_FILE=tests/docker/env.test.example
PRISM_API_PORT=$(grep PRISM_API_PORT "$ENV_FILE" | cut -d'=' -f2)
FRONTEND_PORT=$(grep FRONTEND_PORT "$ENV_FILE" | cut -d'=' -f2)
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

tests/docker/wait-for.sh https://localhost:${FRONTEND_PORT}/healthz 60

API_BASE=http://localhost:${PRISM_API_PORT} k6 run tests/perf/k6/api_mixed_read_write.js --summary-export=k6-summary.json

