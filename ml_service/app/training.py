"""Train the crop and fertilizer models from the bundled CSVs.

The service trains in-process at startup instead of loading pickled artifacts, so no file that can
execute code on load is ever read. Both models are scored with stratified cross-validation, and
training refuses to return a model whose mean accuracy is below its threshold.
"""

from __future__ import annotations

import json
import logging
from dataclasses import asdict, dataclass, field
from pathlib import Path

import numpy as np
import pandas as pd
import sklearn
from sklearn.compose import ColumnTransformer
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import StratifiedKFold, cross_val_score
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import OrdinalEncoder

logger = logging.getLogger("farm_fusion.ml.training")

SERVICE_DIR = Path(__file__).resolve().parent.parent
DATA_DIR = SERVICE_DIR / "data"
BOUNDS_PATH = SERVICE_DIR / "feature_bounds.json"

CROP_FEATURES = ["N", "P", "K", "temperature", "humidity", "ph", "rainfall"]
CROP_LABEL = "label"

FERTILIZER_NUMERIC = ["Temparature", "Humidity", "Soil Moisture", "Nitrogen", "Potassium", "Phosphorous"]
FERTILIZER_CATEGORICAL = ["Soil Type", "Crop Type"]
FERTILIZER_FEATURES = [
    "Temparature",
    "Humidity",
    "Soil Moisture",
    "Soil Type",
    "Crop Type",
    "Nitrogen",
    "Potassium",
    "Phosphorous",
]
FERTILIZER_LABEL = "Fertilizer Name"

# Minimum mean cross-validated accuracy. Measured on the bundled CSVs with scikit-learn 1.9:
# crop 0.9955 (std 0.003), fertilizer 0.98 (std 0.025). The margins absorb fold-to-fold noise.
MIN_CROP_ACCURACY = 0.97
MIN_FERTILIZER_ACCURACY = 0.90

CV_FOLDS = 5
RANDOM_STATE = 42


class TrainingError(RuntimeError):
    """Raised when the data is malformed or a model misses its accuracy threshold."""


@dataclass
class ModelMetrics:
    rows: int
    cv_folds: int
    accuracy_mean: float
    accuracy_std: float
    min_accuracy: float
    class_support: dict[str, int] = field(default_factory=dict)


@dataclass
class TrainedModels:
    crop: Pipeline
    fertilizer: Pipeline
    metrics: dict[str, ModelMetrics]
    sklearn_version: str = sklearn.__version__

    def report(self) -> dict:
        return {
            "sklearn_version": self.sklearn_version,
            "models": {name: asdict(m) for name, m in self.metrics.items()},
        }


def load_bounds(path: Path = BOUNDS_PATH) -> dict[str, dict[str, tuple[float, float]]]:
    raw = json.loads(path.read_text())
    return {
        model: {name: (float(lo), float(hi)) for name, (lo, hi) in fields.items()}
        for model, fields in raw.items()
        if not model.startswith("_")
    }


def _require_columns(df: pd.DataFrame, columns: list[str], source: str) -> None:
    missing = [c for c in columns if c not in df.columns]
    if missing:
        raise TrainingError(f"{source} is missing required columns: {', '.join(missing)}")
    if df[columns].isna().any().any():
        raise TrainingError(f"{source} has empty values in required columns")


def _evaluate(model: Pipeline, X: pd.DataFrame, y: pd.Series, min_accuracy: float, name: str) -> ModelMetrics:
    support = y.value_counts().sort_index()
    if support.min() < CV_FOLDS:
        raise TrainingError(
            f"{name}: class {support.idxmin()!r} has {support.min()} rows, fewer than {CV_FOLDS} folds"
        )

    folds = StratifiedKFold(n_splits=CV_FOLDS, shuffle=True, random_state=RANDOM_STATE)
    scores = cross_val_score(model, X, y, cv=folds, scoring="accuracy")
    metrics = ModelMetrics(
        rows=len(y),
        cv_folds=CV_FOLDS,
        accuracy_mean=round(float(np.mean(scores)), 4),
        accuracy_std=round(float(np.std(scores)), 4),
        min_accuracy=min_accuracy,
        class_support={str(k): int(v) for k, v in support.items()},
    )
    logger.info(
        "%s model cross-validated accuracy %.4f (std %.4f) over %d rows",
        name,
        metrics.accuracy_mean,
        metrics.accuracy_std,
        metrics.rows,
    )
    if metrics.accuracy_mean < min_accuracy:
        raise TrainingError(
            f"{name}: accuracy {metrics.accuracy_mean:.4f} is below the required {min_accuracy:.2f}"
        )
    return metrics


def _forest() -> RandomForestClassifier:
    return RandomForestClassifier(n_estimators=100, random_state=RANDOM_STATE, n_jobs=1)


def train_crop(df: pd.DataFrame) -> tuple[Pipeline, ModelMetrics]:
    _require_columns(df, CROP_FEATURES + [CROP_LABEL], "crop data")
    X, y = df[CROP_FEATURES], df[CROP_LABEL].astype(str)
    model = Pipeline([("classifier", _forest())])
    metrics = _evaluate(model, X, y, MIN_CROP_ACCURACY, "crop")
    model.fit(X, y)
    return model, metrics


def train_fertilizer(df: pd.DataFrame) -> tuple[Pipeline, ModelMetrics]:
    _require_columns(df, FERTILIZER_FEATURES + [FERTILIZER_LABEL], "fertilizer data")
    X, y = df[FERTILIZER_FEATURES], df[FERTILIZER_LABEL].astype(str)
    # The encoder sits inside the pipeline, so each CV fold fits it on that fold's training rows only.
    encode = ColumnTransformer(
        [
            (
                "categorical",
                OrdinalEncoder(handle_unknown="use_encoded_value", unknown_value=-1),
                FERTILIZER_CATEGORICAL,
            )
        ],
        remainder="passthrough",
    )
    model = Pipeline([("encode", encode), ("classifier", _forest())])
    metrics = _evaluate(model, X, y, MIN_FERTILIZER_ACCURACY, "fertilizer")
    model.fit(X, y)
    return model, metrics


def train_all(data_dir: Path = DATA_DIR) -> TrainedModels:
    crop_model, crop_metrics = train_crop(pd.read_csv(data_dir / "crop_recommendation.csv"))
    fert_model, fert_metrics = train_fertilizer(pd.read_csv(data_dir / "fertilizer_recommendation.csv"))
    return TrainedModels(
        crop=crop_model,
        fertilizer=fert_model,
        metrics={"crop": crop_metrics, "fertilizer": fert_metrics},
    )
