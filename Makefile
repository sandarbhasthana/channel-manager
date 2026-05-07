.PHONY: build test lint proto migrate-up migrate-down migrate-version docker-up docker-down dev api seed

# Load .env into the environment for any target that needs DB_* / DATABASE_URL.
ifneq (,$(wildcard .env))
include .env
export
endif

# ── Build ────────────────────────────────────────────────────────────────────
build:
	go build -o bin/api ./apps/api
	go build -o bin/sync-worker ./apps/sync-worker

# ── Test ─────────────────────────────────────────────────────────────────────
test:
	go test ./...

# ── Lint ─────────────────────────────────────────────────────────────────────
lint:
	golangci-lint run ./...

# ── Proto ────────────────────────────────────────────────────────────────────
proto:
	buf generate

# ── Migrations ───────────────────────────────────────────────────────────────
migrate-up:
	go run ./platform/db/cmd/migrate up

migrate-down:
	go run ./platform/db/cmd/migrate down

migrate-version:
	go run ./platform/db/cmd/migrate version

# ── Docker ───────────────────────────────────────────────────────────────────
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# ── Dev ──────────────────────────────────────────────────────────────────────
dev:
	cd apps/dashboard && pnpm dev

# Runs the API server. .env is loaded at the top of this Makefile so all
# DB_*, WORKOS_*, and OTEL_* variables are already in the environment.
api:
	cd apps/api && go run .

# ── Seed ─────────────────────────────────────────────────────────────────────
seed:
	@echo "TODO: implement database seeding"
