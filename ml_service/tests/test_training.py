import pandas as pd
import pytest

from app import training
from app.training import TrainingError, load_bounds


def test_bounds_cover_training_data():
    bounds = load_bounds()
    crop = pd.read_csv(training.DATA_DIR / "crop_recommendation.csv")
    fert = pd.read_csv(training.DATA_DIR / "fertilizer_recommendation.csv")
    fert_columns = {
        "temperature": "Temparature",
        "humidity": "Humidity",
        "soil_moisture": "Soil Moisture",
        "nitrogen": "Nitrogen",
        "potassium": "Potassium",
        "phosphorous": "Phosphorous",
    }

    for name, (lo, hi) in bounds["crop"].items():
        assert lo <= crop[name].min() and crop[name].max() <= hi, name
    for name, (lo, hi) in bounds["fertilizer"].items():
        col = fert[fert_columns[name]]
        assert lo <= col.min() and col.max() <= hi, name
    assert bounds["fertilizer"]["nitrogen"][1] == fert["Nitrogen"].max()
    assert bounds["fertilizer"]["soil_moisture"][0] == fert["Soil Moisture"].min()


def test_missing_column_is_rejected():
    df = pd.read_csv(training.DATA_DIR / "fertilizer_recommendation.csv").drop(columns=["Soil Type"])
    with pytest.raises(TrainingError, match="Soil Type"):
        training.train_fertilizer(df)


def test_accuracy_gate_refuses_weak_model():
    df = pd.read_csv(training.DATA_DIR / "fertilizer_recommendation.csv")
    df["Fertilizer Name"] = df["Fertilizer Name"].sample(frac=1, random_state=0).to_numpy()
    with pytest.raises(TrainingError, match="below the required"):
        training.train_fertilizer(df)


def test_rare_class_below_fold_count_is_rejected():
    df = pd.read_csv(training.DATA_DIR / "fertilizer_recommendation.csv")
    keep = df[df["Fertilizer Name"] != "10-26-26"]
    rare = df[df["Fertilizer Name"] == "10-26-26"].head(2)
    with pytest.raises(TrainingError, match="fewer than"):
        training.train_fertilizer(pd.concat([keep, rare]))


def test_report_lists_every_class():
    models = training.train_all()
    report = models.report()["models"]
    assert len(report["crop"]["class_support"]) == 22
    assert report["fertilizer"]["class_support"]["17-17-17"] == 7
    assert report["fertilizer"]["accuracy_mean"] >= training.MIN_FERTILIZER_ACCURACY
