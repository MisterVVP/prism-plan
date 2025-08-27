#!/bin/bash
set -euo pipefail
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

COMPOSE="docker compose -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap "$COMPOSE down -v" EXIT

tests/docker/wait-for.sh http://localhost/healthz

(cd tests/integration && go test ./...)
