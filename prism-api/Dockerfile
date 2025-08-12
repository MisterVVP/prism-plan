FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o handler .

FROM mcr.microsoft.com/azure-functions/base:4
WORKDIR /home/site/wwwroot
COPY --from=build /src/handler ./handler
COPY host.json ./
COPY function.json ./
ENV AzureWebJobsScriptRoot=/home/site/wwwroot \
    AzureFunctionsJobHost__Logging__Console__IsEnabled=true \
    FUNCTIONS_CUSTOMHANDLER_PORT=7071
EXPOSE 7071
