.PHONY: solver backend frontend dev start db-create migrate-up migrate-down migrate-install

# One-time setup: install the golang-migrate CLI (needs Go + the postgres build tag).
migrate-install:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Creates the employee_scheduler database inside the already-running
# `local-postgres` container (postgres:18 on :5432). Safe to re-run.
db-create:
	docker exec local-postgres psql -U postgres -c "CREATE DATABASE employee_scheduler" || true

migrate-up:
	migrate -path backend/migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path backend/migrations -database "$$DATABASE_URL" down

solver:
	cd solver-service && uv run uvicorn scheduler_api.main:app --reload --port 8080

backend:
	cd backend && set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/api

frontend:
	cd frontend && npm run dev

# Runs all three dev servers concurrently in one terminal (interleaved logs).
# Prefer running `make solver` / `make backend` / `make frontend` in separate
# terminals when you want to watch one service's output cleanly.
dev:
	$(MAKE) -j3 solver backend frontend

start: dev
