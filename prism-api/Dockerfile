FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o prism-api .


FROM alpine
WORKDIR /app
COPY --from=build /src/prism-api ./prism-api
EXPOSE 80
ENTRYPOINT ["./prism-api"]