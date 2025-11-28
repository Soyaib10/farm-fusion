from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager
import joblib

# Global variables to store models
models = {}

@asynccontextmanager
async def lifespan(app: FastAPI):
    # Load models on startup
    print("Loading models...")
    models["crop"] = joblib.load("app/models/crop_model.pkl")
    models["fertilizer"] = joblib.load("app/models/fertilizer_model.pkl")
    models["soil_encoder"] = joblib.load("app/models/soil_encoder.pkl")
    models["crop_encoder"] = joblib.load("app/models/crop_encoder.pkl")
    print("Models loaded successfully")
    yield
    # Cleanup on shutdown
    models.clear()

app = FastAPI(title="Farm Fusion ML Service", lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

from app.routers import recommendation

app.include_router(recommendation.router)

@app.get("/")
def root():
    return {"status": "ok", "service": "Farm Fusion ML Service"}

@app.get("/health")
def health():
    return {"status": "healthy", "models_loaded": len(models)}
