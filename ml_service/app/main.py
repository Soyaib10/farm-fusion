import logging
import os
from contextlib import asynccontextmanager
from pathlib import Path

import joblib
from fastapi import FastAPI, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware

from app.routers import recommendation

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger("farm_fusion.ml")

MODELS_DIR = Path(__file__).resolve().parent / "models"

MODEL_FILES = {
    "crop": "crop_model.pkl",
    "fertilizer": "fertilizer_model.pkl",
    "soil_encoder": "soil_encoder.pkl",
    "crop_encoder": "crop_encoder.pkl",
}


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.models = {}
    logger.info("Loading ML artifacts from %s", MODELS_DIR)
    for key, filename in MODEL_FILES.items():
        artifact_path = (MODELS_DIR / filename).resolve()
        if artifact_path.parent != MODELS_DIR.resolve() or not artifact_path.is_file():
            raise RuntimeError(f"Required model artifact missing: {filename}")
        app.state.models[key] = joblib.load(artifact_path)

    logger.info("Loaded %d ML artifacts successfully", len(app.state.models))
    yield
    app.state.models.clear()


app = FastAPI(
    title="Farm Fusion ML Service",
    docs_url=None if os.getenv("ENV") == "production" else "/docs",
    redoc_url=None,
    lifespan=lifespan,
)

raw_origins = [o.strip() for o in os.getenv("CORS_ALLOWED_ORIGINS", "").split(",") if o.strip()]
if raw_origins and "*" not in raw_origins:
    app.add_middleware(
        CORSMiddleware,
        allow_origins=raw_origins,
        allow_credentials=False,
        allow_methods=["GET", "POST"],
        allow_headers=["Content-Type", "X-Internal-API-Key"],
    )

app.include_router(recommendation.router)


@app.get("/")
def root():
    return {"status": "ok", "service": "Farm Fusion ML Service"}


@app.get("/health")
def health():
    loaded = getattr(app.state, "models", {})
    if len(loaded) != len(MODEL_FILES):
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Models not ready",
        )
    return {"status": "healthy", "models_loaded": len(loaded)}
