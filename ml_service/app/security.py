import os
import secrets

from fastapi import Header, HTTPException, Request, status

MIN_API_KEY_LENGTH = 32
EXAMPLE_API_KEY = "replace-with-a-secure-random-32-byte-ml-api-key"


def load_api_key() -> str:
    """Return ML_API_KEY, or raise so the service refuses to start without a real key."""
    key = os.getenv("ML_API_KEY", "").strip()
    if len(key.encode()) < MIN_API_KEY_LENGTH:
        raise RuntimeError(f"ML_API_KEY must be set and at least {MIN_API_KEY_LENGTH} bytes")
    if key == EXAMPLE_API_KEY:
        raise RuntimeError("ML_API_KEY is still the .env.example placeholder")
    return key


def verify_internal_api_key(
    request: Request,
    x_internal_api_key: str | None = Header(default=None, alias="X-Internal-API-Key"),
) -> None:
    expected = getattr(request.app.state, "api_key", None)
    if not expected:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Service not ready",
        )
    if not x_internal_api_key or not secrets.compare_digest(
        x_internal_api_key.encode(), expected.encode()
    ):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or missing internal API key",
        )
