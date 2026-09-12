# Employee Scheduler

Employee Scheduler is a three-service application for managing employees,
availability, staffing requirements, shift schedules, and replacement
candidates.

## Architecture

| Service | Stack | Responsibility | Local port |
| --- | --- | --- | --- |
| `frontend/` | React 19, TypeScript, Vite, Tailwind CSS, TanStack Query/Table | Management UI; communicates only with the backend | `5173` |
| `backend/` | Go, PostgreSQL, hexagonal architecture | Owns persistence and the approval workflow; orchestrates solver calls | `8081` |
| `solver-service/` | Python, FastAPI, OR-Tools CP-SAT | Stateless scheduling, capacity checks, and replacement ranking | `8080` |

```text
Browser -> frontend -> backend -> PostgreSQL
                           |
                           +----> solver-service
```

The frontend and backend have no authentication in the current demo scope.
The solver is an internal service: all `/api/v1/*` requests require an
`X-API-Key` header, while `/healthz` is public for health probes.

## Prerequisites

- Go `1.27`
- Python `3.11+` and [uv](https://docs.astral.sh/uv/)
- Node.js `22` and npm
- PostgreSQL plus the `psql` client
- Docker (optional, for database setup and container builds)

Database migrations are embedded in the backend binary (via
[golang-migrate](https://github.com/golang-migrate/migrate) as a library, not
a separate CLI) — no extra migration tool to install.

## Local Setup

Run all commands in this section from the project root unless noted otherwise.

### 1. Install dependencies

```bash
cd solver-service && uv sync && cd ..
cd frontend && npm ci && cd ..
```

### 2. Configure environment variables

```bash
cp solver-service/.env.example solver-service/.env
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env
```

The committed examples are ready for the default local ports. Change secrets
and connection settings as needed. `SOLVER_API_KEY` in `backend/.env` must match
`API_KEY` in `solver-service/.env`.

#### Solver configuration

| Variable | Default | Description |
| --- | --- | --- |
| `API_KEY` | Required | Shared secret for `X-API-Key`; minimum 8 characters |
| `MAX_TIME_LIMIT_S` | `60` | Maximum solve time a caller may request |
| `NUM_SEARCH_WORKERS` | `4` | CP-SAT worker threads per solve |
| `MAX_CONCURRENT_SOLVES` | `2` | Maximum concurrent solve requests |
| `MAX_BODY_BYTES` | `10000000` | Request size limit checked from `Content-Length` |
| `ENABLE_DOCS` | `false` | Enables unauthenticated `/docs`, `/redoc`, and `/openapi.json` |
| `LOG_LEVEL` | `INFO` | Python log level |

Keep `MAX_CONCURRENT_SOLVES * NUM_SEARCH_WORKERS` close to the solver's CPU
limit. Enable API docs only in a trusted development environment.

#### Backend configuration

| Variable | Default | Description |
| --- | --- | --- |
| `DATABASE_URL` | None | PostgreSQL URL; takes precedence over the discrete database variables |
| `DATABASE_HOST` | Required without `DATABASE_URL` | PostgreSQL host |
| `DATABASE_PORT` | `5432` | PostgreSQL port |
| `DATABASE_USER` | Required without `DATABASE_URL` | PostgreSQL user |
| `DATABASE_PASSWORD` | Empty | PostgreSQL password |
| `DATABASE_NAME` | `employee_scheduler` | PostgreSQL database |
| `DATABASE_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `SOLVER_API_KEY` | Required | Must match the solver's `API_KEY` |
| `SOLVER_BASE_URL` | `http://localhost:8080` | Solver service URL |
| `HTTP_PORT` | `8081` | Backend HTTP port |
| `CORS_ORIGIN` | `http://localhost:5173` | Allowed frontend origin |
| `LOG_LEVEL` | `info` | zap log level (debug/info/warn/error) |
| `DB_AUTO_MIGRATE` | `false` | Run embedded migrations automatically at startup |
| `OTEL_SDK_DISABLED` | `false` | Disable OpenTelemetry tracing entirely |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `lgtm.observability.svc:4317` | OTLP/gRPC endpoint (LGTM stack) |
| `OTEL_SERVICE_NAME` | `employee-scheduler-backend` | Service name reported to traces |

#### Frontend configuration

| Variable | Default | Description |
| --- | --- | --- |
| `VITE_API_BASE_URL` | `http://localhost:8081` | Backend URL; if unset, Vite's local `/api` proxy is used during development |

### 3. Create and migrate the database

The `db-create` target expects a running Docker container named
`local-postgres` with the `postgres` user:

```bash
make db-create
```

Alternatively, create `employee_scheduler` with your preferred PostgreSQL
tool. Then apply the migrations (embedded in the backend binary) with a
URL-encoded password:

```bash
export DATABASE_URL='postgres://postgres:<encoded-password>@localhost:5432/employee_scheduler?sslmode=disable'
make backend-migrate-up
```

This runs `go run ./cmd/api migrate up` under the hood — see
`make backend-migrate-down` / `make backend-migrate-version` for the other
manual ops, or set `DB_AUTO_MIGRATE=true` to have the server apply pending
migrations automatically on startup instead.

Migrations create only the schema. Load the optional development dataset with:

```bash
psql "$DATABASE_URL" -f backend/seeds/demo.sql
```

The seed contains 21 employees, gates A/B/G/D, and a 28-day availability
window anchored to the date on which it is applied.

## Running Locally

Start each service in a separate terminal:

```bash
make solver
make backend
make frontend
```

Or run all three with interleaved logs:

```bash
make start
```

Open <http://localhost:5173>. When `ENABLE_DOCS=true`, the solver's Swagger UI
is available at <http://localhost:8080/docs>.

## Health Check

```bash
curl http://localhost:8080/healthz
curl http://localhost:8081/api/v1/health
curl http://localhost:8081/api/v1/employees
curl -X POST http://localhost:8081/api/v1/capacity-check \
  -H 'Content-Type: application/json' \
  -d '{"num_days":28}'
```

In the UI, create or load employee availability, use **Lịch xếp ca** to solve
and approve a schedule, then use **Đề xuất thay ca** to rank replacement
candidates for a slot.

## API Overview

The browser should use only the backend API. The backend enriches requests with
persisted data before forwarding relevant operations to the solver.

### Backend API

Base URL: `http://localhost:8081/api/v1`

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Service health |
| `GET`, `POST` | `/employees` | List or create employees |
| `PUT`, `DELETE` | `/employees/{employee_id}` | Update or deactivate an employee |
| `POST` | `/employees/{employee_id}/restore` | Restore an employee |
| `PUT` | `/employees/{employee_id}/availability` | Set availability for a date |
| `PUT` | `/employees/{employee_id}/leave` | Set leave status for a date |
| `GET` | `/config` | Get scheduling configuration |
| `PUT` | `/config/gates/{gate_code}/shifts/{shift_type}` | Update staffing requirements |
| `PUT` | `/config/gates/{gate_code}/rename` | Rename a gate |
| `POST` | `/schedule/solve` | Generate a schedule |
| `GET` | `/schedule/latest` | Get the latest schedule |
| `POST` | `/schedule/approve` | Approve assignments |
| `POST` | `/capacity-check` | Compare staffing demand and supply |
| `POST` | `/candidates` | Rank replacement candidates |

### Solver API

Base URL: `http://localhost:8080/api/v1`. These endpoints are intended for the
backend, not the browser.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/solve` | Solve a new schedule or re-solve with locked assignments |
| `POST` | `/capacity-check` | Run a fast demand-versus-supply calculation without CP-SAT |
| `POST` | `/replacement-candidates` | Return ranked candidates for an open slot; never auto-assigns |

Example direct solver request:

```bash
curl -X POST http://localhost:8080/api/v1/capacity-check \
  -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"num_days":28,"employee_count":21}'
```

Every solver request is self-contained because the solver has no database or
persisted state. Omit `config` from a `/solve` request to use the scheduling
defaults defined by the domain model. For incident re-solves, pass approved
cells as `locked_assignments` and leave only the affected area open.

## Tests and Quality Checks

```bash
# Backend unit tests; PostgreSQL integration tests are skipped without TEST_DATABASE_URL
cd backend && go test ./... && cd ..

# Backend tests including PostgreSQL integration tests
cd backend && TEST_DATABASE_URL='postgres://postgres:<encoded-password>@localhost:5432/employee_scheduler_test?sslmode=disable' go test ./... && cd ..

# Solver tests, lint, and formatting
cd solver-service && uv run pytest --cov=src --cov-report=term-missing && cd ..
cd solver-service && uv run ruff check src tests && uv run ruff format --check src tests && cd ..

# Frontend lint and production build
cd frontend && npm run lint && npm run build && cd ..
```

Create, migrate, and seed `employee_scheduler_test` before running the backend
integration tests.

## Container Images

Each service has an independent Docker build context:

```bash
docker build -t employee-scheduler-frontend ./frontend
docker build -t employee-scheduler-backend ./backend
docker build -t employee-scheduler-solver ./solver-service
```

The GitHub Actions workflow tests all services independently, publishes images
to GHCR after successful `main` builds, and dispatches deployment updates to the
separate GitOps repository.

## Project Layout

```text
.
|-- backend/             # Go API, migrations, repositories, and domain services
|-- frontend/            # React management UI
|-- solver-service/      # FastAPI and OR-Tools scheduling engine
|-- .docs/               # Domain notes, backlog, and production-gap analysis
|-- .github/workflows/   # CI and image publishing
|-- Makefile             # Local development and migration commands
`-- README.md            # Unified project documentation
```

Additional design references:

- [MVP business rules](.docs/memo_mvp.md)
- [Production gaps](.docs/mvp_production_gaps.md)
- [Open design questions](.docs/backlog.md)
- [Original scheduling notebook](.docs/shift_scheduler_mvp.ipynb)
