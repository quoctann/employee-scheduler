# Employee Scheduler

Three services:

- **`solver-service/`** — stateless Python FastAPI + OR-Tools CP-SAT solver. Never
  touches a database; called over HTTP with an `X-API-Key` header.
- **`backend/`** — Go, hexagonal architecture (`internal/core/{domain,port,service}`
  + `internal/adapter/{http,postgres,solverclient}`). Owns Postgres persistence
  and orchestrates calls to `solver-service`. No auth — internal demo scope.
- **`frontend/`** — Vite + React + TypeScript + shadcn/ui + Tailwind + TanStack
  Table/Query. Talks only to the Go backend.

See [docs/mvp_production_gaps.md](docs/mvp_production_gaps.md) for what's
intentionally deferred past this demo pass, and
[docs/20260907_solver_plan.md](docs/20260907_solver_plan.md) /
[docs/memo_mvp_solver_coverage.md](docs/memo_mvp_solver_coverage.md) for the
underlying scheduling domain and architecture decisions.

## One-time setup

### 1. Env files

Copy the committed examples, then adjust secrets and local connection details:

```bash
cp solver-service/.env.example solver-service/.env
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env
```

**`solver-service/.env`**
```
API_KEY=local-dev-key
ENABLE_DOCS=true
```

**`backend/.env`**
```
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=change-me
DATABASE_NAME=employee_scheduler
DATABASE_SSLMODE=disable
SOLVER_API_KEY=local-dev-key
SOLVER_BASE_URL=http://localhost:8080
HTTP_PORT=8081
CORS_ORIGIN=http://localhost:5173
```

**`frontend/.env`**
```
VITE_API_BASE_URL=http://localhost:8081
```

(`SOLVER_API_KEY` in `backend/.env` must match `API_KEY` in
`solver-service/.env` — it's the shared secret the Go backend presents to
the solver.)

### 2. Database

Assumes a Postgres server and its `psql`/`createdb` client tools are already
available. Adjust the `DATABASE_*` values in `backend/.env` if yours differs.
The backend also supports `DATABASE_URL` with precedence over discrete values;
`db-create` intentionally requires discrete values so its destination is clear.

```bash
make db-create        # creates the configured database on localhost Postgres
make migrate-install  # one-time: installs the `migrate` CLI (needs Go)
# Migrate using a URL-encoded password (`@` becomes `%40`, for example):
DATABASE_URL='postgres://postgres:<encoded-password>@localhost:5432/employee_scheduler?sslmode=disable' make migrate-up
```

`migrate-up` applies only the schema. Demo data is intentionally separate from
automatic migrations so production deployments start empty. To initialize a
development database once, run:

```bash
psql "$DATABASE_URL" -f backend/seeds/demo.sql
```

The optional dataset contains 21 employees, gates A/B/G/D, and a 28-day
availability window anchored to the day it is applied.

### 3. Frontend dependencies

```bash
cd frontend && npm install
```

## Running it

Three services, three terminals:

```bash
make solver     # solver-service on :8080
make backend    # Go backend on :8081
make frontend   # Vite dev server on :5173
```

(or `make start`, equivalent to `make dev`, to run all three concurrently in one terminal with
interleaved logs). Open **http://localhost:5173**.

## Verifying it works

```bash
curl localhost:8080/healthz                     # {"status":"ok"}
curl localhost:8081/api/v1/health               # {"success":true,"data":{"status":"ok"},...}
curl localhost:8081/api/v1/employees             # 21 seeded employees
curl -X POST localhost:8081/api/v1/capacity-check -H 'Content-Type: application/json' -d '{"num_days":28}'
```

In the UI: **Lịch xếp ca** tab → Solve → pivot table + shortages + summary
render → Approve → **Đề xuất thay ca** tab → pick a slot → ranked candidates
with reasons render.

## Tests

```bash
cd backend && go test ./...                      # unit tests only, no DB needed
TEST_DATABASE_URL=postgres://postgres:1@localhost:5432/employee_scheduler_test?sslmode=disable go test ./...
                                                  # includes Postgres integration tests
                                                  # (create + migrate that DB the same way as above first)
```
