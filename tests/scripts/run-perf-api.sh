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

tests/docker/wait-for.sh "${PRISM_API_LB_BASE}${AZ_FUNC_HEALTH_ENDPOINT}" 30
tests/docker/wait-for.sh "${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT}" 30

K6_VUS=${K6_VUS:-10}
K6_DURATION=${K6_DURATION:-30s}
tokens=$(jq -n '[]')
for i in $(seq 1 "$K6_VUS"); do
  user="perf-user-$i"
  tok=$(cd tests/utils && go run ./cmd/gen-token "$user")
  [ "$i" -eq 1 ] && TEST_BEARER=${TEST_BEARER:-$tok}
  tokens=$(jq --arg value "$tok" '. + [$value]' <<<"$tokens")
done
echo "$tokens" > tests/perf/k6/bearers.json
export TEST_BEARER K6_VUS K6_DURATION PRISM_API_LB_BASE

curl -I ${PRISM_API_LB_BASE}/api/commands # warmup request
k6 run tests/perf/k6/api_heavy_write.js --summary-export=k6-summary-heavy_write.json

curl -I ${PRISM_API_LB_BASE}/api/tasks  # warmup request
k6 run tests/perf/k6/api_heavy_read.js --summary-export=k6-summary-heavy_read.json

k6 run tests/perf/k6/api_mixed_read_write.js --summary-export=k6-summary-mixed_read_write.json