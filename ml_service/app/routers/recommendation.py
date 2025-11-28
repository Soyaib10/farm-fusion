from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field
from typing import Literal
import numpy as np

router = APIRouter(prefix="/predict", tags=["predictions"])

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
    crop_type: Literal["Maize", "Sugarcane", "Cotton", "Tobacco", "Paddy", "Barley", "Wheat", "Millets", "Oil seeds", "Pulses", "Ground Nuts"]
    nitrogen: float = Field(ge=0, le=39, description="Nitrogen content (0-39)")
    potassium: float = Field(ge=0, le=19, description="Potassium content (0-19)")
    phosphorous: float = Field(ge=0, le=42, description="Phosphorous content (0-42)")

@router.post("/crop")
def predict_crop(request: CropPredictionRequest):
    from app.main import models
    
    try:
        features = np.array([[
            request.N, request.P, request.K,
            request.temperature, request.humidity,
            request.ph, request.rainfall
        ]])
        
        prediction = models["crop"].predict(features)[0]
        probabilities = models["crop"].predict_proba(features)[0]
        confidence = float(max(probabilities))
        
        return {
            "crop": prediction,
            "confidence": confidence
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@router.post("/fertilizer")
def predict_fertilizer(request: FertilizerPredictionRequest):
    from app.main import models
    
    try:
        # Encode categorical features
        soil_encoded = models["soil_encoder"].transform([request.soil_type])[0]
        crop_encoded = models["crop_encoder"].transform([request.crop_type])[0]
        
        features = np.array([[
            request.temperature, request.humidity, request.soil_moisture,
            soil_encoded, crop_encoded,
            request.nitrogen, request.potassium, request.phosphorous
        ]])
        
        prediction = models["fertilizer"].predict(features)[0]
        probabilities = models["fertilizer"].predict_proba(features)[0]
        confidence = float(max(probabilities))
        
        return {
            "fertilizer": prediction,
            "confidence": confidence
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
