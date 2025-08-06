FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o read-model-updater .

FROM alpine:latest
WORKDIR /app
COPY --from=build /src/read-model-updater ./
CMD ["./read-model-updater"]

