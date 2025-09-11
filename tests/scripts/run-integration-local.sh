#!/bin/bash
set -euo pipefail

AZURITE_PID=""
DOMAIN_PID=""
RMU_PID=""
STREAM_PID=""
API_PID=""

cleanup() {
  local status=$?
  for pid in "$API_PID" "$STREAM_PID" "$RMU_PID" "$DOMAIN_PID" "$AZURITE_PID"; do
    if [ -n "$pid" ]; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
  exit $status
}
trap cleanup EXIT

ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"/..

# Environment variables similar to tests/docker/env.test but adjusted for localhost services
export VITE_AUTH0_DOMAIN="example.auth0.com"
export VITE_AUTH0_CLIENT_ID=client
export VITE_AUTH0_AUDIENCE="https://api.example.com"
export VITE_STREAM_URL="http://localhost/stream"
export STORAGE_CONNECTION_STRING="DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://127.0.0.1:10000/devstoreaccount1;QueueEndpoint=http://127.0.0.1:10001/devstoreaccount1;TableEndpoint=http://127.0.0.1:10002/devstoreaccount1;"
export STORAGE_CONNECTION_STRING_LOCAL="DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;BlobEndpoint=http://localhost:10000/devstoreaccount1;QueueEndpoint=http://localhost:10001/devstoreaccount1;TableEndpoint=http://localhost:10002/devstoreaccount1;"
export TASK_EVENTS_TABLE=TaskEvents
export USER_EVENTS_TABLE=UserEvents
export TASKS_TABLE=Tasks
export SETTINGS_TABLE=Settings
export USERS_TABLE=Users
export COMMAND_QUEUE=command-queue
export DOMAIN_EVENTS_QUEUE=domain-events
export REDIS_CONNECTION_STRING="redis://localhost:6379"
export TASK_UPDATES_CHANNEL=task-updates
export SETTINGS_UPDATES_CHANNEL=settings-updates
export PRISM_API_PORT=8080
export STREAM_SERVICE_PORT=8090
export CORS_ALLOWED_ORIGINS=".*"
export AZ_FUNC_HEALTH_ENDPOINT="/healthz"
export API_HEALTH_ENDPOINT="/healthz"
export AUTH0_TEST_MODE=1
export TEST_JWT_SECRET=${TEST_JWT_SECRET:-testsecret}
export TEST_POLL_TIMEOUT="20s"
export PRISM_API_BASE="http://localhost:${PRISM_API_PORT}"
export STREAM_SERVICE_BASE="http://localhost:${STREAM_SERVICE_PORT}"
export AzureWebJobsStorage="${STORAGE_CONNECTION_STRING}"
export AzureFunctionsJobHost__Logging__Console__IsEnabled="true"
export FUNCTIONS_WORKER_RUNTIME="custom"

# Start Azurite
npx azurite -s -l --skipApiVersionCheck ./azurite-data --blobHost 127.0.0.1 --queueHost 127.0.0.1 --tableHost 127.0.0.1 &
AZURITE_PID=$!

# Give services time to start
sleep 2

# Initialize storage (tables and queues)
( cd storage-init && go run . >/tmp/storage-init.log 2>&1 )

# Start domain service
if command -v func >/dev/null 2>&1; then
  (
    cd domain-service/src/DomainService.FunctionApp && \
    FUNCTIONS_WORKER_RUNTIME=dotnet-isolated func start --port 7071 >/tmp/domain-service.log 2>&1
  ) &
  DOMAIN_PID=$!
else
  echo "func not installed; skipping domain-service" >&2
  DOMAIN_PID=""
fi

# Start read-model-updater
(
  cd read-model-updater/az-funcs && \
  cp ../host.json . && \
  func start --port 7072 >/tmp/read-model-updater.log 2>&1
) &
RMU_PID=$!

# Start stream service
( cd stream-service && go run . >/tmp/stream-service.log 2>&1 ) &
STREAM_PID=$!

# Start prism API
( cd prism-api && go run . >/tmp/prism-api.log 2>&1 ) &
API_PID=$!

# Wait for APIs to be reachable
./tests/docker/wait-for.sh ${PRISM_API_BASE}${AZ_FUNC_HEALTH_ENDPOINT} 60
./tests/docker/wait-for.sh ${STREAM_SERVICE_BASE}${API_HEALTH_ENDPOINT} 60

# Run integration tests
cd tests/integration
GO111MODULE=on go test ./...