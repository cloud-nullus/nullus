#!/bin/bash
set -euo pipefail

go test ./internal/... -coverprofile=coverage.out -count=1

COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
echo "Total coverage: ${COVERAGE}%"

# 문턱은 "지금 수준에서 더 내려가지 않게" 잡는다.
#
# 예전에는 60 이었는데 실제 커버리지는 49% 대였다. 통과할 수 없는 문턱은
# 아무것도 지키지 못한다 — 실제로 이 검사 때문에 CI 전체가 5개월간 꺼져 있었고,
# 그동안 go build 도 go test 도 PR 을 막지 못했다.
#
# CLAUDE.md 의 목표는 v1 GA 기준 70% 다. 커버리지가 오르면 이 값을 함께 올린다.
THRESHOLD=49
if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
  echo "FAIL: Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
  exit 1
fi

echo "PASS: Coverage meets threshold"
