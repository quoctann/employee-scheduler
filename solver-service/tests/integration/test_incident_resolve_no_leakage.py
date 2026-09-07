"""Port of docs/shift_scheduler_mvp.ipynb cells 17-18: an employee calls in sick on one day,
we lock every other (employee, date) cell to the already-approved schedule and re-solve — the
result must differ from the original ONLY on the affected day.
"""

from datetime import date, timedelta

CONFIG = {
    "requirements": {"X": {"sang": {"nv": 1}, "dem": {"nv": 1}}},
    "shift_hours": {"X": {"sang": 8, "dem": 8}},
    "lead_gates": [],
    "target_hours_per_week": 40,
}
START = date(2026, 9, 7)
NUM_DAYS = 5
EMPLOYEE_IDS = [f"NV{i:02d}" for i in range(1, 6)]


def _days() -> list[str]:
    return [(START + timedelta(days=i)).isoformat() for i in range(NUM_DAYS)]


def _base_payload() -> dict:
    days = _days()
    employees = [{"employee_id": eid, "name": eid, "role": "NV"} for eid in EMPLOYEE_IDS]
    availability = {eid: {day: {"sang": True, "dem": True} for day in days} for eid in EMPLOYEE_IDS}
    return {
        "start_date": START.isoformat(),
        "num_days": NUM_DAYS,
        "employees": employees,
        "availability": availability,
        "config": CONFIG,
        "time_limit_s": 10,
    }


def _schedule_grid(schedule: list[dict]) -> dict[tuple[str, str], str]:
    """(employee_id, date) -> "gate-shift" or absent (meaning OFF)."""
    return {(e["employee_id"], e["date"]): f"{e['gate']}-{e['shift']}" for e in schedule}


def test_local_resolve_does_not_change_days_outside_the_incident(client, auth_headers):
    days = _days()
    sick_day = days[2]

    first_payload = _base_payload()
    first_resp = client.post("/api/v1/solve", json=first_payload, headers=auth_headers)
    assert first_resp.status_code == 200
    schedule1 = first_resp.json()["data"]["schedule"]
    grid1 = _schedule_grid(schedule1)

    # find someone actually working on the sick day so the incident has a real effect to verify
    sick_emp = next(eid for (eid, d) in grid1 if d == sick_day)

    locked_assignments = []
    for eid in EMPLOYEE_IDS:
        for day in days:
            if day == sick_day:
                continue
            cell = grid1.get((eid, day))
            if cell is None:
                locked_assignments.append({"employee_id": eid, "date": day, "off": True})
            else:
                gate, shift = cell.split("-")
                locked_assignments.append({"employee_id": eid, "date": day, "gate": gate, "shift": shift})

    second_payload = _base_payload()
    second_payload["availability"][sick_emp][sick_day] = {"sang": False, "dem": False}
    second_payload["locked_assignments"] = locked_assignments

    second_resp = client.post("/api/v1/solve", json=second_payload, headers=auth_headers)
    assert second_resp.status_code == 200
    schedule2 = second_resp.json()["data"]["schedule"]
    grid2 = _schedule_grid(schedule2)

    all_cells = {(eid, day) for eid in EMPLOYEE_IDS for day in days}
    changed = [cell for cell in all_cells if grid1.get(cell) != grid2.get(cell)]
    leaked = [cell for cell in changed if cell[1] != sick_day]

    assert leaked == [], f"changes leaked outside the incident day: {leaked}"
    # the sick employee must no longer be on their old slot for that day
    assert grid2.get((sick_emp, sick_day)) != grid1.get((sick_emp, sick_day))
