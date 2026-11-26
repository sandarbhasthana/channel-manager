.PHONY: build test lint proto migrate-up migrate-down migrate-version docker-up docker-down dev api worker seed

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
# go.work lists modules but the repo root has no go.mod; test each workspace
# module explicitly so `make test` works from the repository root.
GO_WORK_MODULES := $(shell grep -E '^[[:space:]]+\./' go.work | sed 's/^[[:space:]]*//')

test:
	@set -e; for mod in $(GO_WORK_MODULES); do \
		echo "==> testing $$mod"; \
		go test ./$$mod/...; \
	done

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

# Runs the sync worker (OTA booking pulls every 5m, redelivery every 1m).
# Needs SYNC_WORKER_TOKEN in .env, the same value the API runs with.
worker:
	cd apps/sync-worker && go run .

# ── Seed ─────────────────────────────────────────────────────────────────────
seed:
	@echo "TODO: implement database seeding"
