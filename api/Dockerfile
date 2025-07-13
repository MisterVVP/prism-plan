FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o handler .

FROM alpine:latest
WORKDIR /app
COPY --from=build /src/handler ./
# Copy function metadata so container mirrors Azure setup (not strictly required)
COPY host.json ./
COPY tasks ./tasks
ENV FUNCTIONS_CUSTOMHANDLER_PORT=7071
EXPOSE 7071
CMD ["./handler"]
