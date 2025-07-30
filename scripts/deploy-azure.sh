#!/bin/bash
set -e
RESOURCE_GROUP="prism-plan-rg"
LOCATION="westeurope"
STORAGE="prismplanstorage"
FUNCAPP="prism-plan-api"

cd frontend
npm run build
cd ..
az group create --name $RESOURCE_GROUP --location $LOCATION
az storage account create --name $STORAGE --resource-group $RESOURCE_GROUP --sku Standard_LRS
az storage blob service-properties update --static-website --account-name $STORAGE --index-document index.html
az storage blob upload-batch -s ./dist -d \$web --account-name $STORAGE
az functionapp create --resource-group $RESOURCE_GROUP --consumption-plan-location $LOCATION \
  --runtime custom --functions-version 4 --name $FUNCAPP --storage-account $STORAGE
cd api
GOOS=linux GOARCH=amd64 go build -o handler .
func azure functionapp publish $FUNCAPP
