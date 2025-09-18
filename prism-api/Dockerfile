FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o handler .

FROM mcr.microsoft.com/azure-functions/base:4
WORKDIR /home/site/wwwroot
RUN mkdir -p /home/data/Functions/secrets
COPY --from=build /src/handler ./handler
COPY host.json ./
COPY az-funcs/tasks ./tasks
COPY az-funcs/settings ./settings
COPY az-funcs/commands ./commands
COPY az-funcs/healthz ./healthz
COPY secrets/host.json /home/data/Functions/secrets/host.json
