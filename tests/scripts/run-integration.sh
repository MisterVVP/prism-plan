#!/bin/bash
set -euo pipefail
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

REPO_ROOT=$(pwd)
ENV_FILE="$REPO_ROOT/tests/docker/env.test"

if [[ -n "${MSYSTEM:-}" ]]; then
  excludes="API_HEALTH_ENDPOINT;"
  if [[ -n "${MSYS2_ENV_CONV_EXCL:-}" ]]; then
    export MSYS2_ENV_CONV_EXCL="${MSYS2_ENV_CONV_EXCL};${excludes}"
  else
    export MSYS2_ENV_CONV_EXCL="$excludes"
  fi
fi

set -a
# shellcheck source=tests/docker/env.test
source "$ENV_FILE"
set +a
ls
COMPOSE=(
  docker compose
  --env-file "$ENV_FILE"
  -f "$REPO_ROOT/docker-compose.yml"
  -f "$REPO_ROOT/tests/docker/docker-compose.tests.yml"
)

cleanup() {
  "${COMPOSE[@]}" down -v
}
trap cleanup EXIT

"${COMPOSE[@]}" up -d

tests/docker/wait-for.sh "${PRISM_API_LB_BASE}${API_HEALTH_ENDPOINT}" 30
tests/docker/wait-for.sh "${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT}" 30

pushd tests/integration > /dev/null
go test ./...
popd > /dev/null

