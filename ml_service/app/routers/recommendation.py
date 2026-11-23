import logging
from typing import Literal

import numpy as np
import pandas as pd
from fastapi import APIRouter, Depends, HTTPException, Request, status
from pydantic import BaseModel, Field

from app.security import verify_internal_api_key
from app.training import CROP_FEATURES, FERTILIZER_FEATURES, load_bounds

logger = logging.getLogger("farm_fusion.ml.recommendation")

BOUNDS = load_bounds()


def bounded(model: str, name: str, label: str):
    lo, hi = BOUNDS[model][name]
    return Field(ge=lo, le=hi, description=f"{label} ({lo:g}-{hi:g})")


router = APIRouter(
    prefix="/predict",
    tags=["predictions"],
    dependencies=[Depends(verify_internal_api_key)],
)


class CropPredictionRequest(BaseModel):
    N: float = bounded("crop", "N", "Nitrogen content")
    P: float = bounded("crop", "P", "Phosphorus content")
    K: float = bounded("crop", "K", "Potassium content")
    temperature: float = bounded("crop", "temperature", "Temperature in Celsius")
    humidity: float = bounded("crop", "humidity", "Humidity percentage")
    ph: float = bounded("crop", "ph", "pH value")
    rainfall: float = bounded("crop", "rainfall", "Rainfall in mm")


class FertilizerPredictionRequest(BaseModel):
    temperature: float = bounded("fertilizer", "temperature", "Temperature in Celsius")
    humidity: float = bounded("fertilizer", "humidity", "Humidity percentage")
    soil_moisture: float = bounded("fertilizer", "soil_moisture", "Soil moisture")
    soil_type: Literal["Sandy", "Loamy", "Black", "Red", "Clayey"]
    crop_type: Literal[
        "Maize",
        "Sugarcane",
        "Cotton",
        "Tobacco",
        "Paddy",
        "Barley",
        "Wheat",
        "Millets",
        "Oil seeds",
        "Pulses",
        "Ground Nuts",
    ]
    nitrogen: float = bounded("fertilizer", "nitrogen", "Nitrogen content")
    potassium: float = bounded("fertilizer", "potassium", "Potassium content")
    phosphorous: float = bounded("fertilizer", "phosphorous", "Phosphorous content")


def _trained(request: Request):
    models = getattr(request.app.state, "models", None)
    if models is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Prediction models unavailable",
        )
    return models


def _rank(model, features: pd.DataFrame, key: str) -> dict:
    probabilities = model.predict_proba(features)[0]
    classes = model.classes_
    order = np.argsort(probabilities)[::-1]
    best = order[0]
    confidence = float(probabilities[best])

    response: dict = {
        key: str(classes[best]),
        "confidence": confidence,
        "alternatives": [
            {key: str(classes[i]), "confidence": float(probabilities[i])} for i in order[:3]
        ],
    }
    if confidence < 0.6:
        response["warning"] = "Low confidence prediction. Consider multiple options."
    return response


@router.post("/crop")
def predict_crop(payload: CropPredictionRequest, request: Request):
    models = _trained(request)
    try:
        features = pd.DataFrame(
            [[payload.N, payload.P, payload.K, payload.temperature, payload.humidity, payload.ph, payload.rainfall]],
            columns=CROP_FEATURES,
        )
        return _rank(models.crop, features, "crop")
    except Exception:
        logger.exception("Crop prediction failed")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Internal prediction error",
        )


@router.post("/fertilizer")
def predict_fertilizer(payload: FertilizerPredictionRequest, request: Request):
    models = _trained(request)
    try:
        features = pd.DataFrame(
            [[
                payload.temperature,
                payload.humidity,
                payload.soil_moisture,
                payload.soil_type,
                payload.crop_type,
                payload.nitrogen,
                payload.potassium,
                payload.phosphorous,
            ]],
            columns=FERTILIZER_FEATURES,
        )
        return _rank(models.fertilizer, features, "fertilizer")
    except Exception:
        logger.exception("Fertilizer prediction failed")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Internal prediction error",
        )
