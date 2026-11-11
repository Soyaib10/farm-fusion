import logging
import os
import secrets
from typing import Literal

import numpy as np
import pandas as pd
from fastapi import APIRouter, Depends, Header, HTTPException, Request, status
from pydantic import BaseModel, Field

logger = logging.getLogger("farm_fusion.ml.recommendation")


def verify_internal_api_key(
    x_internal_api_key: str | None = Header(default=None, alias="X-Internal-API-Key"),
) -> None:
    expected = os.getenv("ML_API_KEY", "").strip()
    if not expected:
        return
    if not x_internal_api_key or not secrets.compare_digest(x_internal_api_key, expected):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid or missing internal API key",
        )


router = APIRouter(
    prefix="/predict",
    tags=["predictions"],
    dependencies=[Depends(verify_internal_api_key)],
)


class CropPredictionRequest(BaseModel):
    N: float = Field(ge=0, le=140, description="Nitrogen content (0-140)")
    P: float = Field(ge=5, le=145, description="Phosphorus content (5-145)")
    K: float = Field(ge=5, le=205, description="Potassium content (5-205)")
    temperature: float = Field(ge=8, le=44, description="Temperature in Celsius (8-44)")
    humidity: float = Field(ge=14, le=100, description="Humidity percentage (14-100)")
    ph: float = Field(ge=3.5, le=10, description="pH value (3.5-10)")
    rainfall: float = Field(ge=20, le=300, description="Rainfall in mm (20-300)")


class FertilizerPredictionRequest(BaseModel):
    temperature: float = Field(ge=25, le=38, description="Temperature in Celsius (25-38)")
    humidity: float = Field(ge=50, le=72, description="Humidity percentage (50-72)")
    soil_moisture: float = Field(ge=10, le=65, description="Soil moisture (10-65)")
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
    nitrogen: float = Field(ge=0, le=39, description="Nitrogen content (0-39)")
    potassium: float = Field(ge=0, le=19, description="Potassium content (0-19)")
    phosphorous: float = Field(ge=0, le=42, description="Phosphorous content (0-42)")


@router.post("/crop")
def predict_crop(payload: CropPredictionRequest, request: Request):
    models = getattr(request.app.state, "models", None)
    if not models or "crop" not in models:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Crop prediction model unavailable",
        )

    try:
        features = pd.DataFrame(
            [[
                payload.N,
                payload.P,
                payload.K,
                payload.temperature,
                payload.humidity,
                payload.ph,
                payload.rainfall,
            ]],
            columns=["N", "P", "K", "temperature", "humidity", "ph", "rainfall"],
        )

        crop_model = models["crop"]
        prediction = crop_model.predict(features)[0]
        probabilities = crop_model.predict_proba(features)[0]
        confidence = float(max(probabilities))

        classes = crop_model.classes_
        top_3_indices = np.argsort(probabilities)[-3:][::-1]
        alternatives = [
            {"crop": str(classes[i]), "confidence": float(probabilities[i])}
            for i in top_3_indices
        ]

        response: dict = {
            "crop": str(prediction),
            "confidence": confidence,
            "alternatives": alternatives,
        }
        if confidence < 0.6:
            response["warning"] = "Low confidence prediction. Consider multiple options."

        return response
    except Exception:
        logger.exception("Crop prediction failed")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Internal prediction error",
        )


@router.post("/fertilizer")
def predict_fertilizer(payload: FertilizerPredictionRequest, request: Request):
    models = getattr(request.app.state, "models", None)
    if not models or "fertilizer" not in models:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Fertilizer prediction model unavailable",
        )

    try:
        soil_encoded = models["soil_encoder"].transform([payload.soil_type])[0]
        crop_encoded = models["crop_encoder"].transform([payload.crop_type])[0]

        features = pd.DataFrame(
            [[
                payload.temperature,
                payload.humidity,
                payload.soil_moisture,
                soil_encoded,
                crop_encoded,
                payload.nitrogen,
                payload.potassium,
                payload.phosphorous,
            ]],
            columns=[
                "Temparature",
                "Humidity",
                "Soil Moisture",
                "Soil Type",
                "Crop Type",
                "Nitrogen",
                "Potassium",
                "Phosphorous",
            ],
        )

        fert_model = models["fertilizer"]
        prediction = fert_model.predict(features)[0]
        probabilities = fert_model.predict_proba(features)[0]
        confidence = float(max(probabilities))

        classes = fert_model.classes_
        top_3_indices = np.argsort(probabilities)[-3:][::-1]
        alternatives = [
            {"fertilizer": str(classes[i]), "confidence": float(probabilities[i])}
            for i in top_3_indices
        ]

        response: dict = {
            "fertilizer": str(prediction),
            "confidence": confidence,
            "alternatives": alternatives,
        }
        if confidence < 0.6:
            response["warning"] = "Low confidence prediction. Consider multiple options."

        return response
    except Exception:
        logger.exception("Fertilizer prediction failed")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Internal prediction error",
        )
