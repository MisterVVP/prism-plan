#!/bin/bash
set -e
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

ENV_FILE=tests/docker/env.test
set -a
# shellcheck source=tests/docker/env.test
source "$ENV_FILE"
set +a
COMPOSE="docker compose --env-file $ENV_FILE -f docker-compose.yml -f tests/docker/docker-compose.tests.yml"
$COMPOSE up -d
trap '$COMPOSE down -v' EXIT

STREAM_SERVICE_BASE=http://localhost:${STREAM_SERVICE_PORT}
PRISM_API_BASE=http://localhost:${PRISM_API_PORT}

tests/docker/wait-for.sh "${PRISM_API_BASE}${AZ_FUNC_HEALTH_ENDPOINT}" 60
tests/docker/wait-for.sh "${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT}" 60

K6_VUS=${K6_VUS:-10}
K6_DURATION=${K6_DURATION:-30s}
if [ -z "$TEST_BEARERS" ]; then
  tokens=""
  for i in $(seq 1 $K6_VUS); do
    user="perf-user-$i"
    tok=$(cd tests/utils && go run ./cmd/gen-token "$user")
    [ "$i" -eq 1 ] && first_token="$tok"
    tokens="$tokens\"$i\":\"$tok\""
    [ "$i" -lt "$K6_VUS" ] && tokens="$tokens,"
  done
  TEST_BEARERS="{${tokens}}"
  TEST_BEARER=${TEST_BEARER:-$first_token}
fi
export TEST_BEARER TEST_BEARERS K6_VUS K6_DURATION
k6 run tests/perf/k6/api_mixed_read_write.js --summary-export=k6-summary.json

