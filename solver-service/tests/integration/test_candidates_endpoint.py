from datetime import date, timedelta

CONFIG = {
    "requirements": {"X": {"sang": {"nv": 1}, "dem": {"nv": 1}}},
    "shift_hours": {"X": {"sang": 8, "dem": 8}},
    "lead_gates": [],
    "target_hours_per_week": 40,
}


def payload() -> dict:
    start = date(2026, 9, 7)
    days = [(start + timedelta(days=i)).isoformat() for i in range(5)]
    employees = [
        {"employee_id": "NV01", "name": "NV01", "role": "NV"},
        {"employee_id": "NV02", "name": "NV02", "role": "NV"},
    ]
    availability = {e["employee_id"]: {day: {"sang": True, "dem": True} for day in days} for e in employees}
    return {
        "start_date": start.isoformat(),
        "num_days": 5,
        "employees": employees,
        "availability": availability,
        "current_schedule": [],
        "target_slot": {"date": days[2], "gate": "X", "shift": "sang"},
        "top_n": 5,
        "config": CONFIG,
    }


def test_replacement_candidates_requires_auth(client):
    resp = client.post("/api/v1/replacement-candidates", json=payload())
    assert resp.status_code == 401


def test_replacement_candidates_happy_path(client, auth_headers):
    resp = client.post("/api/v1/replacement-candidates", json=payload(), headers=auth_headers)

    assert resp.status_code == 200
    body = resp.json()
    assert body["success"] is True
    candidates = body["data"]["candidates"]
    assert {c["employee_id"] for c in candidates} == {"NV01", "NV02"}
    for c in candidates:
        assert c["reasons"]
