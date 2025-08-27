#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

docker compose -f docker-compose.yml -f tests/docker/docker-compose.tests.yml up -d
tests/docker/wait-for.sh http://localhost/healthz
go test ./tests/integration/...
docker compose -f docker-compose.yml -f tests/docker/docker-compose.tests.yml down -v
