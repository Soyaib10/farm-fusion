#!/usr/bin/env bash
set -euo pipefail

if [ ! -f .env ]; then
  echo "ERROR: .env not found. Run: cp .env.example .env and fill in values."
  exit 1
fi

# Safely load .env without word-splitting or glob expansion
set -a
# shellcheck disable=SC1091
source .env
set +a

API_PID=""
SCHEDULER_PID=""
WORKER_PID=""
ML_PID=""

log() { echo "[$(date '+%H:%M:%S')] $*"; }

cleanup() {
  log "Shutting down services..."
  for pid in "$API_PID" "$SCHEDULER_PID" "$WORKER_PID" "$ML_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  wait 2>/dev/null || true
  docker compose stop
}
trap cleanup EXIT INT TERM

# 1. Start infrastructure via Docker
log "Starting infrastructure (postgres, redis, rabbitmq)..."
docker compose up -d

log "Waiting for services to be ready..."
until pg_isready -d "$DATABASE_URL" -q 2>/dev/null; do sleep 1; done
REDIS_HOST="${REDIS_ADDR%%:*}"
REDIS_PORT="${REDIS_ADDR##*:}"
until redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping > /dev/null 2>&1; do sleep 1; done
until nc -z 127.0.0.1 5672 2>/dev/null; do sleep 1; done
log "Infrastructure ready."

# 2. Run database migrations idempotently via schema_migrations table
log "Running migrations..."
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

for f in migrations/*.up.sql; do
  version="$(basename "$f")"
  already_applied="$(psql "$DATABASE_URL" -tAc "SELECT 1 FROM schema_migrations WHERE version = '$version'" | tr -d '[:space:]')"
  if [ "$already_applied" != "1" ]; then
    log "  Applying $version..."
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f" -q
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q -c "INSERT INTO schema_migrations (version) VALUES ('$version')"
  fi
done

# 3. Start ML service (bound to loopback)
log "Starting ML service..."
(
  cd ml_service
  [ ! -d venv ] && python3 -m venv venv
  # shellcheck disable=SC1091
  source venv/bin/activate
  pip install -q -r requirements.txt
  [ ! -f app/models/crop_model.pkl ] && python train_models.py
  exec uvicorn app.main:app --host 127.0.0.1 --port 8000 --log-level warning
) &
ML_PID=$!

for i in $(seq 1 20); do
  curl -sf http://127.0.0.1:8000/health > /dev/null 2>&1 && break
  [ "$i" -eq 20 ] && { echo "ERROR: ML service failed to start."; exit 1; }
  sleep 1
done
log "ML service ready."

# 4. Build Go binaries
log "Building Go binaries..."
mkdir -p bin
go build -o bin/api ./cmd/api
go build -o bin/scheduler ./cmd/scheduler
go build -o bin/worker ./cmd/worker

# 5. Start services
SERVER_PORT="${SERVER_PORT:-8080}"
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
log "  API:       http://127.0.0.1:${SERVER_PORT}"
log "  Health:    http://127.0.0.1:${SERVER_PORT}/healthz"
log "  ML:        http://127.0.0.1:8000"
log "  Scheduler: PID $SCHEDULER_PID"
log "  Worker:    PID $WORKER_PID"

wait "$API_PID"
