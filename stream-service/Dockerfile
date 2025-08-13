FROM golang:1.24-alpine AS build
WORKDIR /src/stream-service
COPY stream-service/go.mod stream-service/go.sum ./
COPY prism-api ../prism-api
RUN go mod download
COPY stream-service .
RUN go build -o stream-service .

FROM alpine
WORKDIR /app
COPY --from=build /src/stream-service/stream-service ./stream-service
EXPOSE 80
CMD ["./stream-service"]
