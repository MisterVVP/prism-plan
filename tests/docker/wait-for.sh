#!/bin/bash
set -euo pipefail

url="$1"
timeout="${2:-60}"

echo "Waiting for $url ..."
headers=()
if [[ -n "${PRISM_API_FUNCTION_KEY:-}" && -n "${PRISM_API_LB_BASE:-}" && "$url" == ${PRISM_API_LB_BASE}* ]]; then
  headers+=(-H "x-functions-key: ${PRISM_API_FUNCTION_KEY}")
fi
for ((i=0; i<timeout; i++)); do
  if curl -kfs "${headers[@]}" "$url" >/dev/null; then
    exit 0
  fi
  sleep 1
done

echo "Timeout waiting for $url" >&2
exit 1
