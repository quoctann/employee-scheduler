.PHONY: solver backend frontend dev start db-create backend-migrate-up backend-migrate-down backend-migrate-version

# Creates the employee_scheduler database inside the already-running
# `local-postgres` container (postgres:18 on :5432). Safe to re-run.
db-create:
	docker exec local-postgres psql -U postgres -c "CREATE DATABASE employee_scheduler" || true

# Migrations are embedded in the api binary (see backend/internal/platform/migrate)
# and normally run automatically at startup when DB_AUTO_MIGRATE=true. These
# targets are for manual/local ops.
backend-migrate-up:
	cd backend && set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/api migrate up

backend-migrate-down:
	cd backend && set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/api migrate down

backend-migrate-version:
	cd backend && set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/api migrate version

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
