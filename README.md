# Prism plan

This is a **minimal but functional prototype** of the four‑lane task board with colour + shape categories and a Dockerised static host.

---
## 🚀 Quick start (dev)
```bash
# 1. Install deps
npm install
# 2. Copy env vars and start Vite dev server (http://localhost:5173)
cp .env.example .env
npm run dev
```

## 🐳 Build & run with Docker
```bash
# Build production bundle & nginx image
docker build -t time-manager .
# Serve on http://localhost:8080
docker run --rm -p 8080:80 time-manager
```

### Local API with Docker Compose
```bash
# Build and run the frontend and Go API
cp api/.env.example api/.env  # update with your storage creds
docker-compose up --build
```

## 📦 Environment variables
The frontend expects an API base URL. Set `VITE_API_BASE_URL` along with the Auth0 variables in `.env`.
An example is provided in `.env.example`.

The backend is an Azure Function written in Go using the Echo framework and Azure Table Storage. Place the storage connection string and table name in `api/.env` based on `api/.env.example`.

## ☁️ Deploying to Azure (free tiers)
1. Build the static site:
   ```bash
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
