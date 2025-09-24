#!/bin/bash
set -euo pipefail
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..


ENV_FILE=tests/docker/env.test
set -a
# shellcheck source=tests/docker/env.test
source "$ENV_FILE"
set +a
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

tests/docker/wait-for.sh "${PRISM_API_LB_BASE}${AZ_FUNC_HEALTH_ENDPOINT}" 30
tests/docker/wait-for.sh "${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT}" 30

cd tests/integration && go test ./...

