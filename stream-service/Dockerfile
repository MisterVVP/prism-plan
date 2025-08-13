FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o stream-service .

FROM alpine
WORKDIR /app
COPY --from=build /src/stream-service ./stream-service
EXPOSE 80
CMD ["./stream-service"]
