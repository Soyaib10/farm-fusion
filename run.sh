#!/usr/bin/env bash
set -e

# Load .env
if [ ! -f .env ]; then
  echo "ERROR: .env not found. Run: cp .env.example .env and fill in values."
  exit 1
fi
export $(grep -v '^#' .env | grep -v '^$' | xargs)

# ── helpers ──────────────────────────────────────────────────────────────────
log() { echo "[$(date '+%H:%M:%S')] $*"; }
cleanup() {
  log "Shutting down..."
  kill "$API_PID" "$SCHEDULER_PID" "$WORKER_PID" "$ML_PID" 2>/dev/null || true
  wait 2>/dev/null
  docker compose stop
}
trap cleanup EXIT INT TERM

# ── 1. Start infrastructure via Docker ───────────────────────────────────────
log "Starting infrastructure (postgres, redis, rabbitmq)..."
docker compose up -d

log "Waiting for services to be ready..."
until pg_isready -d "$DATABASE_URL" -q 2>/dev/null; do sleep 1; done
until redis-cli -h "${REDIS_ADDR%%:*}" -p "${REDIS_ADDR##*:}" ping > /dev/null 2>&1; do sleep 1; done
until nc -z localhost 5672 2>/dev/null; do sleep 1; done
log "Infrastructure ready."

# ── 2. Run migrations ─────────────────────────────────────────────────────────
log "Running migrations..."
for f in migrations/*.up.sql; do
  psql "$DATABASE_URL" -f "$f" -q
done

# ── 3. Start ML service ───────────────────────────────────────────────────────
log "Starting ML service..."
cd ml_service
[ ! -d venv ] && python3 -m venv venv
source venv/bin/activate
pip install -q fastapi uvicorn scikit-learn pandas numpy joblib
[ ! -f models/crop_model.pkl ] && python train_models.py
uvicorn app.main:app --host 0.0.0.0 --port 8000 --log-level warning &
ML_PID=$!
cd ..

# Wait for ML service
for i in $(seq 1 15); do
  curl -sf http://localhost:8000/health > /dev/null 2>&1 && break
  [ "$i" -eq 15 ] && { echo "ERROR: ML service failed to start."; exit 1; }
  sleep 1
done
log "ML service ready."

# ── 4. Build Go binaries ──────────────────────────────────────────────────────
log "Building..."
go build -o bin/api cmd/api/main.go
go build -o bin/scheduler cmd/scheduler/main.go
go build -o bin/worker cmd/worker/main.go

# ── 5. Start services ─────────────────────────────────────────────────────────
log "Starting API server on :${SERVER_PORT}..."
./bin/api &
API_PID=$!

log "Starting worker..."
./bin/worker &
WORKER_PID=$!

log "Starting scheduler..."
./bin/scheduler &
SCHEDULER_PID=$!

log "All services running. Press Ctrl+C to stop."
log "  API:       http://localhost:${SERVER_PORT}"
log "  ML:        http://localhost:8000"
log "  Scheduler: PID $SCHEDULER_PID"
log "  Worker:    PID $WORKER_PID"

wait "$API_PID"
