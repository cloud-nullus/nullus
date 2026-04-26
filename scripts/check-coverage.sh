#!/bin/bash
set -euo pipefail

go test ./internal/... -coverprofile=coverage.out -count=1

COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
echo "Total coverage: ${COVERAGE}%"

THRESHOLD=60
if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
  echo "FAIL: Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
  exit 1
fi

echo "PASS: Coverage meets threshold"
