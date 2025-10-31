#!/bin/bash
set -euo pipefail
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

REPO_ROOT=$(pwd)
DEFAULT_ENV_FILE="tests/docker/env.test"
ENV_FILE=""
AZURITE=false

usage() {
  echo "Usage: $0 [<env-file>] [--azurite]" >&2
  exit 2
}

nonflag_seen=""
for arg in "${@:-}"; do
  if [[ "$arg" == "--azurite" ]]; then
    AZURITE=true
  elif [[ "$arg" == "--help" || "$arg" == "-h" ]]; then
    usage
  elif [[ -z "$nonflag_seen" ]]; then
    ENV_FILE="$arg"
    nonflag_seen="yes"
  else
    echo "Unexpected extra argument: $arg" >&2
    usage
  fi
done

ENV_FILE="${ENV_FILE:-$DEFAULT_ENV_FILE}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Environment file '$ENV_FILE' not found. Provide a valid env file path." >&2
  exit 1
fi

set -a
# shellcheck source=tests/docker/env.test
source "$ENV_FILE"
set +a

COMPOSE=(
  docker compose
  --env-file "$ENV_FILE"
  -f "$REPO_ROOT/docker-compose.yml"
  -f "$REPO_ROOT/tests/docker/docker-compose.tests.yml"
)

if $AZURITE; then
  echo "Azurite exclusive mode is enabled"
  COMPOSE+=(-f "$REPO_ROOT/azurite.yml")
fi

RESULT_DIR="tests/perf/results"
SUMMARY_FILE_REL="$RESULT_DIR/task_request_metrics.json"
SUMMARY_FILE="$(pwd)/$SUMMARY_FILE_REL"
ARTIFACT_DIR="${ARTIFACTS_DIR:-${CI_ARTIFACTS_DIR:-}}"

ensure_artifact_dir() {
  if [[ -n "${ARTIFACT_DIR:-}" ]]; then
    mkdir -p "$ARTIFACT_DIR"
  fi
}

collect_logs_and_teardown() {
  local exit_code=$?
  set +e

  if [[ ${#COMPOSE[@]} -gt 0 ]]; then
    if [[ -n "${SUMMARY_FILE:-}" ]]; then
      echo "Collecting task request observability events..."
      if "${COMPOSE[@]}" logs --no-color --no-log-prefix prism-api-1 prism-api-2 prism-api-3 prism-api-4 prism-api-5 \
        | (cd tests/utils && go run ./cmd/collect-otel-events -out "$SUMMARY_FILE"); then
        echo "Aggregated task metrics saved to $SUMMARY_FILE_REL"
        if [[ -n "${ARTIFACT_DIR:-}" ]]; then
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

    "${COMPOSE[@]}" down -v
  fi

  return "$exit_code"
}

trap collect_logs_and_teardown EXIT

"${COMPOSE[@]}" up -d

tests/docker/wait-for.sh "${PRISM_API_LB_BASE}${API_HEALTH_ENDPOINT}" 30
tests/docker/wait-for.sh "${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT}" 30

PRISM_K6_ARRIVAL_RATE=${PRISM_K6_ARRIVAL_RATE:-10}
PRISM_K6_TIME_UNIT=${PRISM_K6_TIME_UNIT:-1s}
PRISM_K6_DURATION=${PRISM_K6_DURATION:-30s}
PRISM_K6_PRE_ALLOCATED_VUS=${PRISM_K6_PRE_ALLOCATED_VUS:-200}
PRISM_K6_MAX_VUS=${PRISM_K6_MAX_VUS:-1000}

if [[ "$PRISM_K6_MAX_VUS" -gt 10000 ]]; then
  echo "Capping PRISM_K6_MAX_VUS to 10000 (was $PRISM_K6_MAX_VUS)" >&2
  PRISM_K6_MAX_VUS=10000
fi

if [[ "$PRISM_K6_MAX_VUS" -lt "$PRISM_K6_PRE_ALLOCATED_VUS" ]]; then
  PRISM_K6_MAX_VUS=$PRISM_K6_PRE_ALLOCATED_VUS
fi

PRISM_K6_TASK_PAGE_SIZE=${PRISM_K6_TASK_PAGE_SIZE:-}
if [[ -z "$PRISM_K6_TASK_PAGE_SIZE" && -n "${TASKS_PAGE_SIZE:-}" ]]; then
  PRISM_K6_TASK_PAGE_SIZE=$TASKS_PAGE_SIZE
fi

unset K6_ARRIVAL_RATE K6_TIME_UNIT K6_DURATION K6_PRE_ALLOCATED_VUS K6_MAX_VUS K6_TASK_PAGE_SIZE K6_VUS

echo "Generating API tokens for up to $PRISM_K6_MAX_VUS virtual users..."
tokens_file="tests/perf/k6/bearers.json"
tokens_file_abs="$(pwd)/$tokens_file"
mkdir -p "$(dirname "$tokens_file")"
TEST_BEARER=$(cd tests/utils && go run ./cmd/gen-token \
  -count "$PRISM_K6_MAX_VUS" \
  -prefix perf-user \
  -output "$tokens_file_abs")

if [[ -z "${TEST_BEARER:-}" ]]; then
  echo "Failed to capture bearer token" >&2
  exit 1
fi

export TEST_BEARER \
  PRISM_K6_ARRIVAL_RATE \
  PRISM_K6_TIME_UNIT \
  PRISM_K6_DURATION \
  PRISM_K6_PRE_ALLOCATED_VUS \
  PRISM_K6_MAX_VUS \
  PRISM_API_LB_BASE \
  PRISM_K6_TASK_PAGE_SIZE

k6 run tests/perf/k6/api_heavy_write.js --summary-export=k6-summary-heavy_write.json

echo "Waiting for command and event queues to drain before heavy read..."
storage_conn="${STORAGE_CONNECTION_STRING_LOCAL:-${STORAGE_CONNECTION_STRING:-}}"

if [[ -n "$storage_conn" ]]; then
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
# k6 run tests/perf/k6/api_heavy_write_batch.js --summary-export=k6-summary-heavy_write_batch.json

mkdir -p "$RESULT_DIR"
