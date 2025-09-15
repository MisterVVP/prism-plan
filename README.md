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

Fetch tasks from `/api/tasks` and post commands to `/api/commands`.

## Testing

See [tests/README.md](tests/README.md) for running integration and performance tests.

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
2. Deploy this project to Azure/GCP or AWS on free tier. Budget infra costs to 10 EUR per month.
3. Observability setup would've been beneficial, consider adding wide events or traces
4. Frontend code can be revisited and re-factored
   - Improve web UX on mobile devices
   - If we introduce new events that can't be merged/upserted - don't forget to refactor frontend and stream-service
5. Add some basic features to better utilise event sourcing (e.g. undo/redo)
6. Try out to replace azure functions with AWS lambdas and/or GCP Cloud Run functions. Check whether they work better locally and cost less when deployed and scaled out

### Accepted risks
1. Edge case scenario where 2 events contain equal timestamp in nanoseconds is not handled. Probability of such event is extremely low and (for now) it's considered to be out of scope.
   - If required, the problem could be solved by adding additional checks for other fields, storing more granular timestamps or implementing retry events. Right now the read-model-updater simply returns error.
2. Relying on the API node’s clock still carries some risk: if two instances drift even slightly, a later command processed by a skewed node could be dropped as “stale.” Given the serverless environment and cloud‑provider NTP sync, you may choose to accept this trade‑off, but it does leave a small window where legitimate updates could be discarded.
   - Risk is accepted because we trust cloud providers clock sync and planning to run API on serverless compute. If required, this problem can be solved by replacing timestamps with sequences stored in one of our storages or configure all infra to sync with a single NTP, e.g. (AWS one)[https://aws.amazon.com/about-aws/whats-new/2022/11/amazon-time-sync-internet-public-ntp-service/]

### Know issues
1. Perf tests fail due to azurite scalability problems. This is expected and good, because I'm not planning to set up paid azure storage account for this