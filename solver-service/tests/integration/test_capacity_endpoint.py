def test_capacity_check_requires_auth(client):
    resp = client.post("/api/v1/capacity-check", json={"num_days": 28, "employee_count": 21})
    assert resp.status_code == 401


def test_capacity_check_happy_path_matches_notebook_numbers(client, auth_headers):
    resp = client.post(
        "/api/v1/capacity-check",
        json={"num_days": 28, "employee_count": 21},
        headers=auth_headers,
    )

    assert resp.status_code == 200
    body = resp.json()
    assert body["success"] is True
    data = body["data"]
    assert data["demand_hours_per_day"] == 140
    assert data["target_hours_per_employee"] == 176
    assert data["is_sufficient"] is False
