from pathlib import Path

import joblib
import pandas as pd
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import LabelEncoder

BASE_DIR = Path(__file__).resolve().parent
DATA_DIR = BASE_DIR / "data"
MODELS_DIR = BASE_DIR / "app" / "models"


def train_crop_model() -> None:
    print("Training crop recommendation model...")
    df = pd.read_csv(DATA_DIR / "crop_recommendation.csv")

    X = df[["N", "P", "K", "temperature", "humidity", "ph", "rainfall"]]
    y = df["label"]

    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42
    )

    model = RandomForestClassifier(n_estimators=100, random_state=42)
    model.fit(X_train, y_train)

    accuracy = model.score(X_test, y_test)
    print(f"Crop model accuracy: {accuracy:.2%}")

    MODELS_DIR.mkdir(parents=True, exist_ok=True)
    output_path = MODELS_DIR / "crop_model.pkl"
    joblib.dump(model, output_path)
    print(f"Crop model saved to {output_path}")


def train_fertilizer_model() -> None:
    print("\nTraining fertilizer recommendation model...")
    df = pd.read_csv(DATA_DIR / "fertilizer_recommendation.csv")

    le_soil = LabelEncoder()
    le_crop = LabelEncoder()

    df["Soil Type"] = le_soil.fit_transform(df["Soil Type"])
    df["Crop Type"] = le_crop.fit_transform(df["Crop Type"])

    X = df[
        [
            "Temparature",
            "Humidity",
            "Soil Moisture",
            "Soil Type",
            "Crop Type",
            "Nitrogen",
            "Potassium",
            "Phosphorous",
        ]
    ]
    y = df["Fertilizer Name"]

    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42
    )

    model = RandomForestClassifier(n_estimators=100, random_state=42)
    model.fit(X_train, y_train)

    accuracy = model.score(X_test, y_test)
    print(f"Fertilizer model accuracy: {accuracy:.2%}")

    MODELS_DIR.mkdir(parents=True, exist_ok=True)
    joblib.dump(model, MODELS_DIR / "fertilizer_model.pkl")
    joblib.dump(le_soil, MODELS_DIR / "soil_encoder.pkl")
    joblib.dump(le_crop, MODELS_DIR / "crop_encoder.pkl")
    print(f"Fertilizer model and encoders saved to {MODELS_DIR}")


if __name__ == "__main__":
    train_crop_model()
    train_fertilizer_model()
    print("\n✓ All models trained successfully!")
