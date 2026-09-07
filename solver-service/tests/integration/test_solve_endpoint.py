from datetime import date, timedelta

from fastapi.testclient import TestClient

from scheduler_api.api.v1.solve import _solve_semaphore
from scheduler_api.config import settings
from scheduler_api.main import app

SMALL_CONFIG = {
    "requirements": {"X": {"sang": {"nv": 1}, "dem": {"nv": 1}}},
    "shift_hours": {"X": {"sang": 8, "dem": 8}},
    "lead_gates": [],
    "target_hours_per_week": 40,
}


def small_payload(num_days: int = 3, num_employees: int = 3) -> dict:
    start = date(2026, 9, 7)
    days = [(start + timedelta(days=i)).isoformat() for i in range(num_days)]
    employees = [
        {"employee_id": f"NV{i:02d}", "name": f"NV{i:02d}", "role": "NV"} for i in range(1, num_employees + 1)
    ]
    availability = {e["employee_id"]: {day: {"sang": True, "dem": True} for day in days} for e in employees}
    return {
        "start_date": start.isoformat(),
        "num_days": num_days,
        "employees": employees,
        "availability": availability,
        "config": SMALL_CONFIG,
        "time_limit_s": 5,
    }


def test_healthz_does_not_require_auth(client):
    resp = client.get("/healthz")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


def test_solve_without_api_key_returns_401(client):
    resp = client.post("/api/v1/solve", json=small_payload())
    assert resp.status_code == 401
    body = resp.json()
    assert body["success"] is False
    assert body["data"] is None


def test_solve_with_wrong_api_key_returns_401(client):
    resp = client.post("/api/v1/solve", json=small_payload(), headers={"X-API-Key": "wrong"})
    assert resp.status_code == 401


def test_solve_happy_path(client, auth_headers):
    resp = client.post("/api/v1/solve", json=small_payload(), headers=auth_headers)

    assert resp.status_code == 200
    body = resp.json()
    assert body["success"] is True
    assert body["data"]["status"] in ("OPTIMAL", "FEASIBLE")
    assert isinstance(body["data"]["schedule"], list)
    assert len(body["data"]["employee_summary"]) == 3


def test_solve_validation_error_returns_422_envelope(client, auth_headers):
    payload = small_payload()
    del payload["start_date"]

    resp = client.post("/api/v1/solve", json=payload, headers=auth_headers)

    assert resp.status_code == 422
    body = resp.json()
    assert body["success"] is False
    assert body["error"] is not None


def test_solve_referential_integrity_error_returns_400(client, auth_headers):
    payload = small_payload()
    payload["locked_assignments"] = [{"employee_id": "GHOST", "date": payload["start_date"], "off": True}]

    resp = client.post("/api/v1/solve", json=payload, headers=auth_headers)

    assert resp.status_code == 400
    body = resp.json()
    assert body["success"] is False
    assert "GHOST" in body["error"]


def test_solve_understaffed_scenario_returns_200_with_shortages(client, auth_headers):
    payload = small_payload(num_employees=3)
    payload["employees"] = []
    payload["availability"] = {}

    resp = client.post("/api/v1/solve", json=payload, headers=auth_headers)

    assert resp.status_code == 200
    body = resp.json()
    assert body["success"] is True
    assert body["data"]["status"] in ("OPTIMAL", "FEASIBLE")
    assert len(body["data"]["shortages"]) > 0


def test_oversized_content_length_is_rejected_before_parsing(client, auth_headers):
    headers = {**auth_headers, "content-length": str(settings.max_body_bytes + 1)}
    resp = client.post("/api/v1/solve", json=small_payload(), headers=headers)

    assert resp.status_code == 413
    body = resp.json()
    assert body["success"] is False


def test_unhandled_error_returns_500_envelope_without_leaking_details(auth_headers, monkeypatch):
    def boom(**kwargs):
        raise RuntimeError("boom: something internal broke")

    monkeypatch.setattr("scheduler_api.api.v1.solve.solve_schedule", boom)
    # raise_server_exceptions=False: TestClient's default re-raises the handled exception in the
    # test process (for debuggability) even though the server already sent the 500 response —
    # this client mirrors what a real HTTP caller sees instead.
    no_raise_client = TestClient(app, raise_server_exceptions=False)

    resp = no_raise_client.post("/api/v1/solve", json=small_payload(), headers=auth_headers)

    assert resp.status_code == 500
    body = resp.json()
    assert body["success"] is False
    assert "boom" not in body["error"]


def test_solve_returns_503_when_solver_is_at_capacity(client, auth_headers):
    acquired = [_solve_semaphore.acquire(blocking=False) for _ in range(settings.max_concurrent_solves)]
    assert all(acquired), "test assumes the semaphore starts fully available"
    try:
        payload = small_payload()
        payload["time_limit_s"] = 1
        resp = client.post("/api/v1/solve", json=payload, headers=auth_headers)

        assert resp.status_code == 503
        assert resp.json()["success"] is False
    finally:
        for _ in acquired:
            _solve_semaphore.release()
