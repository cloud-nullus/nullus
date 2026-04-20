.PHONY: dev dev-up dev-down dev-status dev-logs build test test-cover test-integration test-golden-path lint migrate-up migrate-down migrate-status web-dev web-build web-test all clean db-shell

DB_URL := postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable
DOCKER_COMPOSE := $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; else echo ""; fi)

ifeq ($(OS),Windows_NT)
GO_BIN_DIR := $(USERPROFILE)/go/bin
MIGRATE_FALLBACK := $(GO_BIN_DIR)/migrate.exe
MIGRATE := $(if $(wildcard $(MIGRATE_FALLBACK)),$(MIGRATE_FALLBACK),migrate.exe)
MIGRATE_CHECK := where migrate
NULL_DEVICE := NUL
else
GO_BIN_DIR := $(HOME)/go/bin
MIGRATE_FALLBACK := $(GO_BIN_DIR)/migrate
MIGRATE := $(shell command -v migrate 2>/dev/null || echo $(MIGRATE_FALLBACK))
MIGRATE_CHECK := command -v migrate
NULL_DEVICE := /dev/null
endif

# ─── 개발 환경 ───
dev-up:
	@test -n "$(DOCKER_COMPOSE)" || (echo "Docker Compose not found. Install docker compose plugin or docker-compose."; exit 1)
	$(DOCKER_COMPOSE) -f docker-compose.dev.yaml up -d
	@echo "Waiting for services..."
	@sleep 3
	@$(DOCKER_COMPOSE) -f docker-compose.dev.yaml ps

dev-down:
	@test -n "$(DOCKER_COMPOSE)" || (echo "Docker Compose not found. Install docker compose plugin or docker-compose."; exit 1)
	$(DOCKER_COMPOSE) -f docker-compose.dev.yaml down

dev-clean:
	@test -n "$(DOCKER_COMPOSE)" || (echo "Docker Compose not found. Install docker compose plugin or docker-compose."; exit 1)
	$(DOCKER_COMPOSE) -f docker-compose.dev.yaml down -v
	@echo "Volumes removed"

dev-status:
	@test -n "$(DOCKER_COMPOSE)" || (echo "Docker Compose not found. Install docker compose plugin or docker-compose."; exit 1)
	$(DOCKER_COMPOSE) -f docker-compose.dev.yaml ps

dev-logs:
	@test -n "$(DOCKER_COMPOSE)" || (echo "Docker Compose not found. Install docker compose plugin or docker-compose."; exit 1)
	$(DOCKER_COMPOSE) -f docker-compose.dev.yaml logs -f --tail=50

dev: dev-up migrate-up
	@echo ""
	@echo "═══════════════════════════════════════════"
	@echo "  Nullus Dev Environment Ready"
	@echo "═══════════════════════════════════════════"
	@echo "  PostgreSQL : localhost:5433"
	@echo "  MinIO      : localhost:9000 (console: 9001)"
	@echo "  Redis      : localhost:6380"
	@echo ""
	@echo "  make build       → Go 빌드"
	@echo "  make run         → API 서버 실행"
	@echo "  make test        → 전체 테스트"
	@echo "  make web-dev     → 프론트엔드 dev 서버"
	@echo "  make db-shell    → psql 접속"
	@echo "═══════════════════════════════════════════"

# ─── 백엔드 ───
build:
	go build -o bin/api ./cmd/api

run: build
	@set -a && [ -f .env.dev ] && . ./.env.dev; \
	NULLUS_DB_HOST=$${NULLUS_DB_HOST:-localhost} \
	NULLUS_DB_PORT=$${NULLUS_DB_PORT:-5433} \
	NULLUS_DB_NAME=$${NULLUS_DB_NAME:-nullus} \
	NULLUS_DB_USER=$${NULLUS_DB_USER:-nullus} \
	NULLUS_DB_PASSWORD=$${NULLUS_DB_PASSWORD:-nullus_dev} \
	NULLUS_DB_SSLMODE=$${NULLUS_DB_SSLMODE:-disable} \
	ENCRYPTION_KEY=$${ENCRYPTION_KEY:-nullus-dev-key-32bytes-padding!!} \
	./bin/api

run-dev: build
	@set -a && [ -f .env.dev ] && . ./.env.dev; \
	ENCRYPTION_KEY=$${ENCRYPTION_KEY:-nullus-dev-key-32bytes-padding!!} \
	./bin/api

test:
	go test ./... -v -count=1

test-integration:
	go test -tags integration ./e2e/ -v -count=1

# F8 Task 6 — Narwhal Golden Path 3종을 실제 로컬 Kind 클러스터 `nullus-platform`
# 에서 순차 배포 검증. Kind 또는 helm CLI 가 없으면 graceful skip.
# 실행 전제: `kind create cluster --name nullus-platform --image kindest/node:v1.30.x`
# 자세한 런북: docs/20_아키텍처/F8_Task6_Kind_Runbook.md
test-golden-path:
	go test -tags e2e -run "^TestF8Task6_GoldenPath" -timeout 60m -v ./e2e/...

test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

# ─── DB ───
migrate-up:
	@$(MIGRATE_CHECK) > $(NULL_DEVICE) 2>&1 || (echo "Installing golang-migrate..." && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)
	$(MIGRATE) -path db/migrations -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE) -path db/migrations -database "$(DB_URL)" down 1

migrate-status:
	$(MIGRATE) -path db/migrations -database "$(DB_URL)" version

db-shell:
	@test -n "$(DOCKER_COMPOSE)" || (echo "Docker Compose not found. Install docker compose plugin or docker-compose."; exit 1)
	$(DOCKER_COMPOSE) -f docker-compose.dev.yaml exec postgres psql -U nullus -d nullus

# ─── 프론트엔드 ───
web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

web-test:
	cd web && npx vitest run

# ─── 전체 ───
all: build web-build test web-test
	@echo "All builds and tests passed"

clean:
	rm -rf bin/ coverage.out coverage.html web/dist
