"""Train both models, enforce the accuracy thresholds, and write a metrics report.

The service trains the same way at startup, so this script writes no model files. Run it to check
the data and to refresh metrics/model_report.json, which the README numbers come from.
"""

import json
import logging
import sys
from pathlib import Path

from app.training import TrainingError, train_all

REPORT_PATH = Path(__file__).resolve().parent / "metrics" / "model_report.json"


def main() -> int:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
    try:
        models = train_all()
    except TrainingError as exc:
        print(f"Training failed: {exc}", file=sys.stderr)
        return 1

    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text(json.dumps(models.report(), indent=2) + "\n")
    print(f"Metrics written to {REPORT_PATH}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
