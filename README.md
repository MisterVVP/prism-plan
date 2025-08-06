# Prism plan

This repository follows an event‑sourced micro‑services architecture.
It includes a thin Prism API, a C# Domain Service (built on .NET 9 preview) for
command processing, and a Go Read‑Model Updater for projections. A mobile client
folder is also provided as a placeholder.

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

The Prism API will automatically create the table if it does not already exist. A self-signed certificate will be generated to serve the frontend over HTTPS.

## 📦 Environment variables
The frontend uses the current origin for API requests, so no extra configuration is needed when the API is served from the same host. Override the API location by setting `VITE_API_BASE_URL` along with the Auth0 variables in `frontend/.env`. With Docker Compose Nginx terminates TLS and proxies `/api` calls to the Go backend so the API is available over HTTPS at `https://localhost:8080/api`.
An example is provided in `frontend/.env.example` for the Auth0 values.
When running with Docker Compose the Auth0 values come from the project `.env` file and are passed as build arguments so the generated bundle calls the local API correctly.
The same `.env` file supplies `AUTH0_DOMAIN` and `AUTH0_AUDIENCE` for the backend so it can verify JWTs.

The Auth0 integration stores tokens in `localStorage` and uses refresh tokens so
the login persists for about an hour even after refreshing the page.

The Prism API is an Azure Function written in Go using the Echo framework and Azure Table Storage. Place the storage connection string and table names for task and user events in `prism-api/.env` based on `prism-api/.env.example`. The example uses the default Azurite credentials so the stack works fully offline. Set `AUTH0_DOMAIN` and `AUTH0_AUDIENCE` so the API can fetch the JWKS from Auth0 and validate incoming tokens. Nginx serving the frontend injects CORS headers, proxies `/api` to the backend and allows all origins by default. Restrict the allowed origins with the `CORS_ALLOWED_ORIGINS` environment variable, which accepts a pipe-separated regular expression.

Use the `TASK_EVENTS_TABLE` and `USER_EVENTS_TABLE` variables to configure where task and user events are stored. Additional tables for other entities can be configured in the same way; the API reuses the same table client for all tables.

The service stores task and user events separately and reconstructs entities on request. Fetch assembled tasks from `/api/tasks` and post events (including user registration) to `/api/events`.

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
