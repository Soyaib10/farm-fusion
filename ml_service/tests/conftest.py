import pytest
from fastapi.testclient import TestClient

API_KEY = "test-key-that-is-at-least-32-bytes-long"


@pytest.fixture(scope="session")
def client():
    mp = pytest.MonkeyPatch()
    mp.setenv("ML_API_KEY", API_KEY)
    from app.main import app

    with TestClient(app) as c:
        yield c
    mp.undo()


@pytest.fixture
def auth_headers():
    return {"X-Internal-API-Key": API_KEY}
