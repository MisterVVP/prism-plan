#!/bin/bash
set -e
url="$1"
until curl -kfs "$url" >/dev/null; do
  sleep 1
done
