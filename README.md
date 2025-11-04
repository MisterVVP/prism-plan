# Prism plan
![main](https://github.com/MisterVVP/prism-plan/actions/workflows/ci.yml/badge.svg?branch=main)

This repository implements an event‑sourced micro‑services architecture.
It includes a thin Prism API, a C# Domain Service (on .NET 9) for command
processing, and a Go Read‑Model Updater for projections. A mobile client folder
is also provided as a placeholder.

## Project Status
The repository is one of my pet projects. Expect breaking changes and other experimental things here.

## Prerequisites
- Docker
- Latest NodeJs, Golang and .NET 9 SDK
- OpenSSL (to generate dev SSL certificates)
- Auth0 account (free tier is fine)

## Local Development with Docker Compose
Make sure to set up env variables in .env file (see .env.example)
> [!TIP]
> use generate-cert.bat for Windows

1. Generate SSL certificates via  scripts/generate-cert.sh or generate-cert.bat or manually
2. ```bash
      docker-compose up --build
   ```

A self-signed certificate will be generated to serve the frontend over HTTPS. Storage tables and queues are provisioned by a `storage-init` service before the other containers start.

### Docker Compose scenarios

Use layered Compose files so each scenario only overrides what differs from the shared stack:

| Scenario | Command |
| --- | --- |
| Local dev (fauxzureq for tables/queues + Azurite for blobs) | `docker compose up --build` |
| CI / test harness (fauxzureq + Azurite, test env vars) | `docker compose -f docker-compose.yml -f tests/docker/docker-compose.tests.yml --env-file tests/docker/env.test up --build` |
| Local Azurite-only stack (queues, tables, blobs) | `docker compose -f docker-compose.yml -f azurite.yml up --build` |

The override files only change the pieces that differ (environment, scaling and the storage emulator). This keeps the shared stack maintainable while still covering all workflows.

Each stack also includes a Python Azure Functions container (`idempotency-cleaner`) that runs on a timer trigger. By default it executes every five minutes to remove completed idempotency rows from the task and user event tables so retries can reuse keys. Override the schedule with the `IDEMPOTENCY_CLEANER_SCHEDULE` environment variable when you need faster feedback (for example, the test compose file runs it every three seconds).

## Custom Azure table and Queue storage emulator
Self-written emulator called fauxzureq is used to replace Azurite in performance tests. Code will be open-source in future, but right now it is too experimental to make public.

## 📦 Environment variables
The frontend uses the current origin for API requests, so no extra configuration is needed when the API is served from the same host. Override the API location by setting `VITE_API_BASE_URL` along with the Auth0 variables in `frontend/.env`. With Docker Compose Nginx terminates TLS and proxies `/api` calls to the Go backend so the API is available over HTTPS at `https://localhost:8080/api`.
An example is provided in `frontend/.env.example` for the Auth0 values.
When running with Docker Compose the Auth0 values come from the project `.env` file and are passed as build arguments so the generated bundle calls the local API correctly.
The same `.env` file supplies `AUTH0_DOMAIN` and `AUTH0_AUDIENCE` for the backend so it can verify JWTs.

The Auth0 integration stores tokens in `localStorage` and uses refresh tokens so
the login persists for about an hour even after refreshing the page.

The Prism API is an Azure Function written in Go using the Echo framework and Azure Storage. It publishes incoming commands to an Azure Queue and serves queries by reading from a denormalised tasks table. Provide the storage connection string, the command queue name and the table used for the read model via environment variables. Set `AUTH0_DOMAIN` and `AUTH0_AUDIENCE` so the API can fetch the JWKS from Auth0 and validate incoming tokens. Nginx serving the frontend injects CORS headers, proxies `/api` to the backend and allows all origins by default. Restrict the allowed origins with the `CORS_ALLOWED_ORIGINS` environment variable, which accepts a pipe-separated regular expression.

Use the following variables to configure storage resources:

- `COMMAND_QUEUE`: queue receiving commands from the API
- `DOMAIN_EVENTS_QUEUE`: queue receiving domain events from the Domain Service
- `TASK_EVENTS_TABLE`: table acting as the event store for tasks
- `TASKS_TABLE`: table containing the read model queried by the API
- `IDEMPOTENCY_CLEANER_SCHEDULE`: CRON expression controlling how frequently the idempotency cleaner runs (defaults to `0 */5 * * * *`)

### Read-model cache configuration

Redis keeps a hot copy of the latest read model data per user to avoid table lookups when serving the first tasks page and the
current settings snapshot. Keys are namespaced as `<userId>:ts` for tasks and `<userId>:us` for settings.

- `TASKS_CACHE_TTL`: expiration for cached task pages (defaults to 12h)
- `SETTINGS_CACHE_TTL`: expiration for cached user settings (defaults to 4h)
- `TASKS_CACHE_SIZE`: number of tasks stored per user in cache. When omitted the value falls back to `TASKS_PAGE_SIZE`; keep the
  two aligned so the cache always holds the first page that Prism API serves without hitting storage.

Fetch tasks from `/api/tasks`—the response includes a `nextPageToken` when more items are available—and post commands to `/api/commands`.

## Testing

See [tests/README.md](tests/README.md) for running integration and performance tests.

To run the API performance suite with a specific Docker Compose environment file, pass the path as the first argument. For local
setups use the bundled `tests/docker/env.test.local` configuration so the containers point to your developer resources:

```bash
bash tests/scripts/run-perf-api.sh tests/docker/env.test.local
```

## ☁️ Deploying to Azure (free tiers)
1. Build the static site:
   ```bash
   cd frontend
   npm run build
   ```
2. Create resources and upload the site:
   ```bash
   az group create --name prism-plan-rg --location westeurope
   az storage account create --name prismplanstorage --resource-group prism-plan-rg --sku Standard_LRS
   az storage blob service-properties update --static-website --account-name prismplanstorage --index-document index.html
   az storage blob upload-batch -s ./dist -d $web --account-name prismplanstorage
   ```
3. Provision the Functions app:
   ```bash
   az functionapp create --resource-group prism-plan-rg --consumption-plan-location westeurope \
     --runtime custom --functions-version 4 --name prism-plan-api --storage-account prismplanstorage
   ```
   Ensure `prism-api/host.json` includes a `customHandler` section pointing to the compiled `handler` executable and that the extension bundle range targets version 4 as shown in this repo.
4. Deploy the API:
   ```bash
   cd prism-api
   GOOS=linux GOARCH=amd64 go build -o handler . # requires Go 1.24+
   func azure functionapp publish prism-plan-api
   ```

The commands above rely on the Azure CLI and the Azure Functions Core Tools. All resources fit within the free tiers.
You can also run `scripts/deploy-azure.sh` to execute the same steps automatically.

## TODO
1. Handle edge-case and error scenarios related to event sourcing and complex design
   - connection and other errors (consider circuit breakers, exponential retries, transactional outbox, sagas and other patterns)
2. Try to replace azure functions with AWS lambdas and/or GCP Cloud Run functions. Check whether they work better locally and cost less when deployed and scaled out
3. Re-populate Redis caches when a miss occurs for the latest tasks page so cold caches do not force repeated table scans.
4. Evaluate how to surface or recover from stale `nextPageToken` values if cached task pages become outdated before TTL expiry (e.g. when multiple concurrent writes happen).
5. Check idempotency handling again (simplest fix is to handle it via message broker, but is it fun to do?)

### Accepted risks
1. Edge case scenario where 2 events contain equal timestamp in nanoseconds is not handled. Probability of such event is extremely low and (for now) it's considered to be out of scope.
   - If required, the problem could be solved by adding additional checks for other fields, storing more granular timestamps or implementing retry events. Right now the read-model-updater simply returns error.
2. Relying on the API node’s clock still carries some risk: if two instances drift even slightly, a later command processed by a skewed node could be dropped as “stale.”
   - If required, this problem can be solved by replacing timestamps with sequences stored in one of our storages or configure all infra to sync with a single NTP, e.g. (AWS one)[https://aws.amazon.com/about-aws/whats-new/2022/11/amazon-time-sync-internet-public-ntp-service/]
