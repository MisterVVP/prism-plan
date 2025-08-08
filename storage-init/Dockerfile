FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN go build -o storage-init .

FROM alpine:latest
WORKDIR /app
COPY --from=build /src/storage-init ./
CMD ["./storage-init"]
