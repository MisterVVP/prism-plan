#!/bin/bash
set -e
url="$1"
until curl -fs "$url" >/dev/null; do
  sleep 1
done
