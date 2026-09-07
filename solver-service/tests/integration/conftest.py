import pytest
from fastapi.testclient import TestClient

from scheduler_api.config import settings
from scheduler_api.main import app


@pytest.fixture
def client() -> TestClient:
    return TestClient(app)


@pytest.fixture
def auth_headers() -> dict[str, str]:
    return {"X-API-Key": settings.api_key}
