#!/bin/bash
set -euo pipefail

url="$1"
timeout="${2:-60}"

echo "Waiting for $url ..."
for ((i=0; i<timeout; i++)); do
  if curl -kfs "$url" >/dev/null; then
    exit 0
  fi
  sleep 1
done

echo "Timeout waiting for $url" >&2
exit 1
