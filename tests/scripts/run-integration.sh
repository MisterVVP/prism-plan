#!/bin/bash
set -euo pipefail
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..


ENV_FILE=tests/docker/env.test
source $ENV_FILE
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

tests/docker/wait-for.sh http://localhost:${PRISM_API_PORT}${AZ_FUNC_HEALTH_ENDPOINT} 60
tests/docker/wait-for.sh http://localhost:${STREAM_SERVICE_PORT}${API_HEALTH_ENDPOINT} 60

STREAM_SERVICE_BASE=http://localhost:${STREAM_SERVICE_PORT}
PRISM_API_BASE=http://localhost:${PRISM_API_PORT} (cd tests/integration && go test ./...)

