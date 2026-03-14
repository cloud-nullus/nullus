.PHONY: dev dev-up dev-down build test lint migrate-up migrate-down web-dev web-build

# 개발 환경
dev-up:
	docker compose -f docker-compose.dev.yaml up -d

dev-down:
	docker compose -f docker-compose.dev.yaml down

dev: dev-up
	@echo "Development environment ready"
	@echo "  PostgreSQL: localhost:5432"
	@echo "  MinIO:      localhost:9000 (console: 9001)"

# 백엔드
build:
	go build -o bin/api ./cmd/api

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

# 마이그레이션
migrate-up:
	migrate -path db/migrations -database "postgres://nullus:nullus_dev@localhost:5432/nullus?sslmode=disable" up

migrate-down:
	migrate -path db/migrations -database "postgres://nullus:nullus_dev@localhost:5432/nullus?sslmode=disable" down 1

# 프론트엔드
web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

web-test:
	cd web && npx vitest run

# 전체
all: build web-build test web-test
	@echo "All builds and tests passed"
