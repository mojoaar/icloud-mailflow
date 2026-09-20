#!/bin/bash
# Regenerate the README screenshots from the demo dataset.
# Usage: bash scripts/screenshots.sh   (optional: THEME=cyberpunk-dark bash scripts/screenshots.sh)
set -euo pipefail
cd "$(dirname "$0")/.."

PORT=8080
BASE="http://127.0.0.1:${PORT}"
BIN=/tmp/mailflow-screenshots
LOG=/tmp/mailflow-screenshots.log

echo "Seeding demo data..."
bash scripts/demo.sh

echo "Building..."
go build -o "$BIN" ./cmd/mailflow

echo "Starting Mailflow on ${BASE}..."
"$BIN" -data=./demo >"$LOG" 2>&1 &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null || true; rm -f "$BIN"' EXIT

for _ in $(seq 1 60); do
  curl -sf "${BASE}/health" >/dev/null 2>&1 && break
  sleep 0.5
done

if ! curl -sf "${BASE}/health" >/dev/null 2>&1; then
  echo "Server did not start; see ${LOG}" >&2
  exit 1
fi

echo "Capturing screenshots..."
BASE_URL="$BASE" node scripts/screenshots.mjs

echo "Done. Screenshots written to docs/screenshots/"
