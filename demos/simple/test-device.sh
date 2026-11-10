#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
if [[ $# -gt 1 ]]; then
  echo "Usage: $0 [thingId]" >&2
  exit 1
fi
export TIO_THING_ID="${1:-${TIO_THING_ID:-test}}"
exec go run ./demos/simple
