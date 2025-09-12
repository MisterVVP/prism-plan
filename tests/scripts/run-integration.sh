#!/bin/bash
set -euo pipefail
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
PRISM_API_LB_BASE=http://localhost:${PRISM_API_LB_PORT}

tests/docker/wait-for.sh ${PRISM_API_LB_BASE}/ 60
tests/docker/wait-for.sh ${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT} 60
cd tests/integration && go test ./...

