.PHONY: dev dev-up dev-down dev-status dev-logs build test test-cover lint migrate-up migrate-down migrate-status web-dev web-build web-test all clean db-shell

DB_URL := postgres://nullus:nullus_dev@localhost:5433/nullus?sslmode=disable

# ─── 개발 환경 ───
dev-up:
	docker compose -f docker-compose.dev.yaml up -d
	@echo "Waiting for services..."
	@sleep 3
	@docker compose -f docker-compose.dev.yaml ps

dev-down:
	docker compose -f docker-compose.dev.yaml down

dev-clean:
	docker compose -f docker-compose.dev.yaml down -v
	@echo "Volumes removed"

dev-status:
	docker compose -f docker-compose.dev.yaml ps

dev-logs:
	docker compose -f docker-compose.dev.yaml logs -f --tail=50

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
	NULLUS_DB_HOST=localhost NULLUS_DB_PORT=5432 NULLUS_DB_NAME=nullus \
	NULLUS_DB_USER=nullus NULLUS_DB_PASSWORD=nullus_dev NULLUS_DB_SSLMODE=disable \
	./bin/api

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

# ─── DB ───
migrate-up:
	@which migrate > /dev/null 2>&1 || (echo "Installing golang-migrate..." && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)
	migrate -path db/migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path db/migrations -database "$(DB_URL)" down 1

migrate-status:
	migrate -path db/migrations -database "$(DB_URL)" version

db-shell:
	docker compose -f docker-compose.dev.yaml exec postgres psql -U nullus -d nullus

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
