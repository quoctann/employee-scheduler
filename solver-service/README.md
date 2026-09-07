# employee-scheduler / scheduler-api

Stateless FastAPI solver service (OR-Tools CP-SAT) for employee shift scheduling. Ported from
[`../.docs/shift_scheduler_mvp.ipynb`](../.docs/shift_scheduler_mvp.ipynb) — see
[`../.docs/memo_mvp.md`](../.docs/memo_mvp.md) for the business rules this implements,
and [`../.docs/backlog.md`](../.docs/backlog.md) for open design questions.

The service holds no state and no database: every request carries all the data needed (employees,
availability, locked/approved assignments, business config). The caller (a Golang backend) owns
persistence and the approve/audit workflow.

## Run locally

```bash
uv sync
cp .env.example .env
# Set API_KEY in .env to a secret with at least 8 characters.
uv run uvicorn scheduler_api.main:app --reload --port 8080
```

Interactive API docs are **off by default** (see `ENABLE_DOCS` below — this service has no
Ingress in front of it and the docs routes aren't behind the API-key check). For local dev:

Set `ENABLE_DOCS=true` in `.env`.

Then: http://127.0.0.1:8080/docs (Swagger UI) or `/openapi.json`.

## Test

```bash
uv run pytest --cov=src --cov-report=term-missing   # 99% coverage as of this writing
uv run ruff check src tests
uv run ruff format --check src tests
```

## Configuration (`.env` or environment variables)

| Variable               | Default   | Meaning                                                                    |
|-------------------------|-----------|------------------------------------------------------------------------------|
| `API_KEY`                | *(required, min 8 chars)* | Value of the `X-API-Key` header on every `/api/v1/*` request. No insecure default — the service fails to start without it. |
| `MAX_TIME_LIMIT_S`        | `60`      | Hard server-side cap on the `time_limit_s` a caller can request.             |
| `NUM_SEARCH_WORKERS`       | `4`       | CP-SAT parallel search workers per solve — keep at/under the pod's CPU limit. |
| `MAX_CONCURRENT_SOLVES`     | `2`       | Max simultaneous `/solve` calls; extras wait, then get `503` after `time_limit_s`. Keep `MAX_CONCURRENT_SOLVES × NUM_SEARCH_WORKERS` close to the CPU limit. |
| `MAX_BODY_BYTES`             | `10_000_000` | Requests with a larger `Content-Length` are rejected with `413` before parsing. |
| `ENABLE_DOCS`                  | `false`   | Serve `/docs`, `/redoc`, `/openapi.json` (unauthenticated by construction — see note above). |
| `LOG_LEVEL`                      | `INFO`    | Python logging level.                                                        |

## Endpoints

Base path `/api/v1`. Header `X-API-Key: <key>` required on all of them; `GET /healthz` does not
require auth (used by k8s liveness/readiness probes).

### `POST /api/v1/solve`

Solves a schedule from scratch, or locally re-solves around an incident via `locked_assignments`
(pass every already-approved cell as locked, leave only the affected day open).

```bash
curl -s -X POST localhost:8080/api/v1/solve \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -d '{
    "start_date": "2026-09-07",
    "num_days": 7,
    "employees": [
      {"employee_id": "NV01", "name": "NV01", "role": "NV"},
      {"employee_id": "NV02", "name": "NV02", "role": "NV"}
    ],
    "availability": {
      "NV01": {"2026-09-07": {"sang": true, "dem": true}},
      "NV02": {"2026-09-07": {"sang": true, "dem": true}}
    },
    "config": {
      "requirements": {"X": {"sang": {"nv": 1}, "dem": {"nv": 1}}},
      "shift_hours": {"X": {"sang": 8, "dem": 8}},
      "lead_gates": []
    }
  }'
```

Omit `config` entirely to use the notebook's real-data defaults (gates A/B/G/D, 44h/week target).

### `POST /api/v1/capacity-check`

Quick demand-vs-supply arithmetic, no CP-SAT — useful before committing to a full solve.

```bash
curl -s -X POST localhost:8080/api/v1/capacity-check \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -d '{"num_days": 28, "employee_count": 21}'
# -> ~140 giờ-người/ngày, cần ~22-23 nhân viên (khớp memo_mvp_solver_coverage.md #1.6)
```

### `POST /api/v1/replacement-candidates`

Top-N ranked replacement suggestions for one open slot (memo item #9b) — a manager picks by hand,
the service never auto-assigns.

```bash
curl -s -X POST localhost:8080/api/v1/replacement-candidates \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -d '{
    "start_date": "2026-09-07", "num_days": 7,
    "employees": [...], "availability": {...}, "current_schedule": [...],
    "target_slot": {"date": "2026-09-09", "gate": "A", "shift": "sang", "requires_lead": false},
    "excluded_employee_id": "NV07", "top_n": 5
  }'
```

`target_slot.requires_lead: true` narrows candidates to whoever is eligible for that gate/shift's
*lead* slot (TC only if `lead_mandatory_role`, else TC or PC) — set it when the person being
replaced was covering the lead requirement, not an ordinary NV slot. `carry_in` (same shape as
`/solve`'s) is optional and only matters when `target_slot.date` is the first day of the horizon —
it prevents suggesting someone who worked the night shift the day before `start_date` for a
`sang` slot, mirroring the solver's own adjacency rule across a rolling-horizon boundary.

## Deploy (k3s, no Ingress)

```bash
docker build -t <registry>/scheduler-api:<tag> .
docker push <registry>/scheduler-api:<tag>
kubectl create secret generic scheduler-api-secrets --from-literal=API_KEY=$(openssl rand -hex 32) -n <namespace>
kubectl apply -f deploy/k8s/ -n <namespace>
```

The Service is `ClusterIP` only, reachable in-cluster at
`http://scheduler-api.<namespace>.svc.cluster.local:8080` — there is no Ingress, this is meant to
be called service-to-service (e.g. from the Golang backend) within the cluster.
[`deploy/k8s/networkpolicy.yaml`](deploy/k8s/networkpolicy.yaml) restricts ingress to that pod
selector — **edit its `matchLabels` to the Golang backend's actual pod labels before applying**,
the placeholder in the file won't match anything by default. The Deployment also runs
non-root/read-only-root-filesystem with all Linux capabilities dropped (the image doesn't write
to disk at runtime — spot-checked with `docker run --read-only`).

## Project layout

```
src/scheduler_api/
├── main.py            # FastAPI app, exception handlers -> {"success","data","error"} envelope
├── config.py           # env-var settings
├── security.py          # X-API-Key dependency
├── api/v1/               # thin HTTP routes — no business logic
├── schemas/                # Pydantic request/response + shared domain models
└── domain/                   # solver.py (CP-SAT), capacity.py, candidate_ranking.py
tests/
├── unit/        # domain layer, run without HTTP
└── integration/  # TestClient hitting the real routes, incl. the incident-resolve regression test
```
