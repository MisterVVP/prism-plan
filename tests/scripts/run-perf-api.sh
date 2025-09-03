#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

ENV_FILE=tests/docker/env.test
set -a
source $ENV_FILE
set +a
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

STREAM_SERVICE_BASE=http://localhost:${STREAM_SERVICE_PORT}
PRISM_API_BASE=http://localhost:${PRISM_API_PORT} 
READ_MODEL_UPDATER_BASE="http://localhost:${READ_MODEL_UPDATER_PORT}"

tests/docker/wait-for.sh ${PRISM_API_BASE}${AZ_FUNC_HEALTH_ENDPOINT} 60
tests/docker/wait-for.sh ${READ_MODEL_UPDATER_BASE}${AZ_FUNC_HEALTH_ENDPOINT} 60
tests/docker/wait-for.sh ${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT} 60

k6 run tests/perf/k6/api_mixed_read_write.js --summary-export=k6-summary.json

