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
RESULT_DIR="tests/perf/results"
SUMMARY_FILE_REL="$RESULT_DIR/task_request_metrics.json"
SUMMARY_FILE="$(pwd)/$SUMMARY_FILE_REL"
ARTIFACT_DIR="${ARTIFACTS_DIR:-${CI_ARTIFACTS_DIR:-}}"

ensure_artifact_dir() {
  if [ -n "${ARTIFACT_DIR:-}" ]; then
    mkdir -p "$ARTIFACT_DIR"
  fi
}

collect_logs_and_teardown() {
  local exit_code=$?
  set +e

  if [ -n "${COMPOSE:-}" ]; then
    if [ -n "${SUMMARY_FILE:-}" ]; then
      echo "Collecting task request observability events..."
      if $COMPOSE logs --no-color --no-log-prefix prism-api-1 prism-api-2 prism-api-3 prism-api-4 prism-api-5 \
        | (cd tests/utils && go run ./cmd/collect-otel-events -out "$SUMMARY_FILE"); then
        echo "Aggregated task metrics saved to $SUMMARY_FILE_REL"
        if [ -n "${ARTIFACT_DIR:-}" ]; then
          ensure_artifact_dir
          artifact_path="$ARTIFACT_DIR/task-request-metrics.json"
          if cp "$SUMMARY_FILE" "$artifact_path"; then
            echo "Task metrics artifact copied to $artifact_path"
          else
            echo "Failed to copy task metrics artifact to $artifact_path" >&2
          fi
        fi
      else
        echo "Failed to collect task metrics from OpenTelemetry logs" >&2
      fi
    fi

    $COMPOSE down -v
  fi

  return "$exit_code"
}

trap collect_logs_and_teardown EXIT

$COMPOSE up -d

tests/docker/wait-for.sh "${PRISM_API_LB_BASE}${API_HEALTH_ENDPOINT}" 30
tests/docker/wait-for.sh "${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT}" 30

K6_VUS=${K6_VUS:-10}
K6_DURATION=${K6_DURATION:-30s}
if [ -z "${K6_TASK_PAGE_SIZE:-}" ] && [ -n "${TASKS_PAGE_SIZE:-}" ]; then
  K6_TASK_PAGE_SIZE=${TASKS_PAGE_SIZE}
else
  K6_TASK_PAGE_SIZE=${K6_TASK_PAGE_SIZE:-${TASKS_PAGE_SIZE:-}}
fi

echo "Generating API tokens..."
tokens="["

for i in $(seq 1 "$K6_VUS"); do
  user="perf-user-$i"
  tok=$(cd tests/utils && go run ./cmd/gen-token "$user")

  [ "$i" -eq 1 ] && TEST_BEARER=${TEST_BEARER:-$tok}

  if [ "$i" -eq "$K6_VUS" ]; then
    tokens="$tokens\"$tok\""
  else
    tokens="$tokens\"$tok\"," 
  fi
done

tokens="$tokens]"
echo "$tokens" > tests/perf/k6/bearers.json

export TEST_BEARER K6_VUS K6_DURATION PRISM_API_LB_BASE K6_TASK_PAGE_SIZE

k6 run tests/perf/k6/api_heavy_write.js --summary-export=k6-summary-heavy_write.json

echo "Waiting for command and event queues to drain before heavy read..."
storage_conn="${STORAGE_CONNECTION_STRING_LOCAL:-${STORAGE_CONNECTION_STRING:-}}"
if [ -n "$storage_conn" ]; then
  if ! (cd tests/utils && go run ./cmd/wait-queues \
    -connection-string "$storage_conn" \
    -queue "${COMMAND_QUEUE}" \
    -queue "${DOMAIN_EVENTS_QUEUE}" \
    -timeout 15m); then
    echo "Queue drain wait failed" >&2
    exit 1
  fi
else
  echo "Storage connection string not available; skipping queue drain wait" >&2
fi

k6 run tests/perf/k6/api_heavy_read.js --summary-export=k6-summary-heavy_read.json

# TODO: can be enabled in future, not used right now
#k6 run tests/perf/k6/api_heavy_write_batch.js --summary-export=k6-summary-heavy_write_batch.json

mkdir -p "$RESULT_DIR"
