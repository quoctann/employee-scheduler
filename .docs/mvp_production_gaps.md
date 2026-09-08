# MVP → Production gap list

This is the backlog of what the current quick-demo MVP (solver-service +
Go backend + React frontend, scaffolded 2026-09-07) deliberately does **not**
cover, so nothing here gets lost or mistaken for an oversight. Pair with
[backlog.txt](./backlog.txt), which tracks the business/domain-level open
questions from the solver design phase — this file is about what's missing
to go from "internal demo" to "production system."

## Security & auth

- **No authentication/authorization** on the Go backend — any client that can
  reach it can call any endpoint. Needs a real auth scheme (session/JWT/OIDC)
  plus per-endpoint authorization once there's more than one class of user
  (employee vs. manager vs. admin).
- **No rate limiting** on any endpoint.
- **Demo Postgres credentials** (`postgres`/`1` in the shared `local-postgres`
  container) are dev-only placeholders baked into `backend/internal/config`'s
  default — production needs real credentials from a secrets manager, never a
  code default.
- **CORS** is a single hardcoded allowed origin (`CORS_ORIGIN` env var) — fine
  for one dev frontend, needs a real per-environment allowlist.
- **Error messages returned to API clients are unsanitized** — DB errors,
  wrapped internal error chains, etc. are passed through as-is
  (`internal/adapter/httpapi/errors.go`). Convenient for local debugging, but
  a production API should return a generic message and log the detail
  server-side only.
- **No audit trail.** `approved_assignments` stores only the *current*
  confirmed state per (employee, date) cell, not who changed what or when —
  see backlog.txt item on approve/audit history. A real deployment needs an
  append-only history table (or at minimum `updated_by`/versioning).
- **No TLS termination** configured in the app itself — assumes a reverse
  proxy/ingress handles HTTPS, which needs to actually exist in whatever
  environment this deploys to.

## Persistence & data model

- **No employee CRUD.** The roster is seeded once via
  `backend/migrations/0002_seed_demo_data.up.sql` and there's no API/UI to
  add, edit, or deactivate an employee yet (the `active` column exists on
  `employees` but nothing toggles it).
- **No weekly incremental availability workflow.** The real business process
  is employees registering availability weekly, before the work week; the
  MVP takes a whole horizon's availability at once from the seed data. See
  backlog.txt item #5.
- **No `leave_type` differentiation** (sick/maternity/annual/unpaid are all
  identical today) — flagged as a possible future business rule split.
- **No retention/archival policy** on `schedule_runs` and its child tables —
  they're append-only and will grow unbounded.
- **No pagination** on `GET /api/v1/employees` or any list-shaped response —
  fine at 21 employees / one run at a time, not fine at real scale.

## Operational readiness

- **No Dockerfile** for the Go backend or the frontend yet (solver-service
  has one; backend/frontend don't) — needed for any containerized deploy.
- **No CI/CD** wired up (build, `go test`, `npm run build`/lint gates).
- **Logging is unstructured** (`log.Printf` in `cmd/api/main.go` and the HTTP
  middleware) — production should use structured logs (e.g. `slog`) with
  request IDs for correlation.
- **No metrics or tracing** (no `/metrics`, no OpenTelemetry spans).
- **`/api/v1/health` doesn't check dependencies** — it's a liveness check,
  not a readiness check; it doesn't verify the Postgres pool or
  solver-service are actually reachable.
- **No contract-versioning strategy** between the Go backend and
  solver-service — they're developed together right now, but nothing
  prevents them drifting apart once they deploy independently.
- **Single shared dev Postgres instance**, no backup/restore or
  point-in-time-recovery story, no connection pooling layer (pgbouncer) for
  when there's more than one backend replica.
- **Solver concurrency isn't coordinated across backend replicas** —
  solver-service's own `MAX_CONCURRENT_SOLVES` semaphore is per-process; if
  the Go backend ever runs with multiple replicas, nothing coordinates how
  many concurrent solves are in flight service-wide.

## Frontend

- **No automated tests** (unit or E2E) — this pass was verified manually
  only, which was communicated as a deliberate scope cut, not an oversight.
- **No global error boundary** — errors surface as per-action toasts only;
  an unexpected render error has no fallback UI.
- **No responsive/mobile pass** — the layout is desktop-oriented (the
  schedule pivot table in particular gets very wide with a full 28-day
  horizon and is not optimized for small screens beyond horizontal scroll).
- **No virtualization** on the schedule pivot table — fine at ~21 rows ×
  28 columns, would need it well before real org scale.
- **Dark mode isn't exposed** — the shadcn theme tokens support it, but
  there's no toggle in the UI.

## Business-workflow gaps (echoed from backlog.txt for one place to scan)

- Weekly-registration UI/workflow: undefined.
- Shortage-review workflow (what a manager does when `shortages` is
  non-empty): undefined.
- Concurrent solve/incident requests: no explicit concurrency control.
- Replacement-candidate ranking weights: implementation-proposed defaults,
  not yet reviewed/approved by the business owner.
- Gate B morning lead requirement ("mandatory" vs. "nice to have"): coded as
  mandatory-with-slack, not confirmed.
- PC (deputy) gate restriction: currently unrestricted (can work any gate);
  unclear if it should mirror TC's gate-B-only restriction.
