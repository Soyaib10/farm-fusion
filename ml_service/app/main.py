import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware

from app.routers import recommendation
from app.security import load_api_key
from app.training import train_all

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger("farm_fusion.ml")


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.api_key = load_api_key()
    app.state.models = None
    logger.info("Training ML models from bundled CSVs")
    app.state.models = train_all()
    logger.info("ML models ready")
    yield
    app.state.models = None


docs_enabled = os.getenv("ENV") != "production"

app = FastAPI(
    title="Farm Fusion ML Service",
    docs_url="/docs" if docs_enabled else None,
    openapi_url="/openapi.json" if docs_enabled else None,
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
    if getattr(app.state, "models", None) is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Models not ready",
        )
    return {"status": "healthy"}
