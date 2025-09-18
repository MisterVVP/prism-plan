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
COPY az-funcs/domain-events ./domain-events
COPY az-funcs/http-domain-events ./http-domain-events
