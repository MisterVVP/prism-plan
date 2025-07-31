# Prism plan


## Local API with Docker Compose
Make sure to set up env variables in .env file (see .env.example)

```bash
docker-compose up --build
```
The API will automatically create the table if it does not already exist.

## 📦 Environment variables
The frontend expects an API base URL. Set `VITE_API_BASE_URL` along with the Auth0 variables in `frontend/.env` for local development.
An example is provided in `frontend/.env.example`.
When running with Docker Compose the Auth0 values come from the project `.env` file and are passed as build arguments so the generated bundle calls the local API correctly.

The Auth0 integration stores tokens in `localStorage` and uses refresh tokens so
the login persists for about an hour even after refreshing the page.

The backend is an Azure Function written in Go using the Echo framework and Azure Table Storage. Place the storage connection string and table name in `api/.env` based on `api/.env.example`. The example uses the default Azurite credentials so the stack works fully offline. CORS is enabled by default so the frontend can call the API from `localhost`.

The service stores task events and reconstructs entities on request. Fetch assembled tasks from `/api/tasks` and post user events to `/api/events`.

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
   Ensure `api/host.json` includes a `customHandler` section pointing to the compiled `handler` executable and that the extension bundle range targets version 4 as shown in this repo.
4. Deploy the API:
   ```bash
   cd api
   GOOS=linux GOARCH=amd64 go build -o handler . # requires Go 1.24+
   func azure functionapp publish prism-plan-api
   ```

The commands above rely on the Azure CLI and the Azure Functions Core Tools. All resources fit within the free tiers.
You can also run `scripts/deploy-azure.sh` to execute the same steps automatically.
