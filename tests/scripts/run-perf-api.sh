#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

docker compose -f docker-compose.yml -f tests/docker/docker-compose.tests.yml up -d
tests/docker/wait-for.sh http://localhost/healthz
k6 run tests/perf/k6/api_mixed_read_write.js --summary-export=k6-summary.json
docker compose -f docker-compose.yml -f tests/docker/docker-compose.tests.yml down -v
