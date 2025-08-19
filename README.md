# Prism plan

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
1. Handle edge-case and error scenarios related to event sourcing
2. Deploy this project to Azure/GCP or AWS on free tier. Budget infra costs to 10 EUR per month.
3. Draw overall system design for this project and ensure it can scale to handle millions of simultaneous users
   - Ideally, PoC should be made locally. However system design is enough with relevant enterprise techs.
   - If PoC is implemented, create load test script to emulate real-world scenario
4. Start system design for mobile app
5. Revisit error handling and triggering logic of stream-service & update system design diagrams
