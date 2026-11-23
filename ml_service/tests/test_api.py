import asyncio

import pytest

from app.main import app as fastapi_app
from app.main import lifespan

CROP = {"N": 90, "P": 42, "K": 43, "temperature": 20.9, "humidity": 82, "ph": 6.5, "rainfall": 203}
FERTILIZER = {
    "temperature": 26,
    "humidity": 52,
    "soil_moisture": 38,
    "soil_type": "Sandy",
    "crop_type": "Maize",
    "nitrogen": 37,
    "potassium": 0,
    "phosphorous": 0,
}


def test_health_reports_ready(client):
    assert client.get("/health").json() == {"status": "healthy"}


@pytest.mark.parametrize("path,body", [("/predict/crop", CROP), ("/predict/fertilizer", FERTILIZER)])
def test_predict_requires_api_key(client, path, body):
    assert client.post(path, json=body).status_code == 401
    assert client.post(path, json=body, headers={"X-Internal-API-Key": "wrong"}).status_code == 401


def test_predict_crop(client, auth_headers):
    resp = client.post("/predict/crop", json=CROP, headers=auth_headers)
    assert resp.status_code == 200
    body = resp.json()
    assert body["crop"] == "rice"
    assert len(body["alternatives"]) == 3
    assert body["alternatives"][0]["crop"] == body["crop"]


def test_predict_fertilizer_accepts_high_nitrogen_from_training_data(client, auth_headers):
    resp = client.post("/predict/fertilizer", json={**FERTILIZER, "nitrogen": 42}, headers=auth_headers)
    assert resp.status_code == 200
    assert resp.json()["fertilizer"] == "Urea"


@pytest.mark.parametrize(
    "override",
    [{"soil_moisture": 10}, {"nitrogen": 43}, {"soil_type": "Gravel"}],
)
def test_predict_fertilizer_rejects_out_of_range(client, auth_headers, override):
    resp = client.post("/predict/fertilizer", json={**FERTILIZER, **override}, headers=auth_headers)
    assert resp.status_code == 422


@pytest.mark.parametrize("key", ["", "short", "replace-with-a-secure-random-32-byte-ml-api-key"])
def test_startup_refuses_weak_api_key(monkeypatch, key):
    monkeypatch.setenv("ML_API_KEY", key)

    async def start():
        async with lifespan(fastapi_app):
            pass

    with pytest.raises(RuntimeError, match="ML_API_KEY"):
        asyncio.run(start())
