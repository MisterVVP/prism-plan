#!/bin/sh
set -e
DIR="$(dirname "$0")/../certs"
mkdir -p "$DIR"
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout "$DIR/nginx.key" -out "$DIR/nginx.crt" \
  -subj "/CN=localhost"
