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

WAIT_TIMEOUT="${WAIT_TIMEOUT:-60}"
RABBITMQ_PORT="${RABBITMQ_PORT:-5672}"
ML_PORT="${ML_PORT:-8000}"

log() { echo "[$(date '+%H:%M:%S')] $*"; }

# wait_for <description> <command...>: retry the command once a second until it succeeds,
# or exit after WAIT_TIMEOUT seconds.
wait_for() {
  local what="$1"
  shift
  local deadline=$((SECONDS + WAIT_TIMEOUT))
  until "$@" > /dev/null 2>&1; do
    if [ "$SECONDS" -ge "$deadline" ]; then
      echo "ERROR: $what not ready after ${WAIT_TIMEOUT}s."
      exit 1
    fi
    sleep 1
  done
}

cleanup() {
  log "Shutting down services..."
  for pid in "$API_PID" "$SCHEDULER_PID" "$WORKER_PID" "$ML_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  wait 2>/dev/null || true
  docker compose stop postgres redis rabbitmq
}
trap cleanup EXIT INT TERM

# 1. Start infrastructure via Docker
log "Starting infrastructure (postgres, redis, rabbitmq)..."
docker compose up -d postgres redis rabbitmq

log "Waiting for services to be ready..."
wait_for "Postgres" pg_isready -d "$DATABASE_URL" -q
REDIS_HOST="${REDIS_ADDR%%:*}"
REDIS_PORT="${REDIS_ADDR##*:}"
redis_ready() {
  REDISCLI_AUTH="${REDIS_PASSWORD:-}" redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping | grep -q PONG
}
wait_for "Redis" redis_ready
wait_for "RabbitMQ on port $RABBITMQ_PORT" nc -z 127.0.0.1 "$RABBITMQ_PORT"
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
  if ! [[ "$version" =~ ^[0-9]+_[A-Za-z0-9_]+\.up\.sql$ ]]; then
    echo "ERROR: unexpected migration filename: $version"
    exit 1
  fi
  # :'version' makes psql quote the value as a literal. Variables only expand in SQL read from stdin or -f.
  already_applied="$(echo "SELECT 1 FROM schema_migrations WHERE version = :'version';" |
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -v version="$version" -tA | tr -d '[:space:]')"
  if [ "$already_applied" != "1" ]; then
    log "  Applying $version..."
    {
      cat "$f"
      echo
      echo "INSERT INTO schema_migrations (version) VALUES (:'version');"
    } | psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -v version="$version" -q --single-transaction
  fi
done

# 3. Start ML service (bound to loopback). It trains its models from the CSVs at startup.
log "Starting ML service..."
(
  cd ml_service
  [ ! -d venv ] && python3 -m venv venv
  # shellcheck disable=SC1091
  source venv/bin/activate
  pip install -q --require-hashes -r requirements.txt
  exec uvicorn app.main:app --host 127.0.0.1 --port "$ML_PORT" --log-level warning
) &
ML_PID=$!

exec 3>&1
ml_ready() {
  kill -0 "$ML_PID" || { echo "ERROR: ML service exited during startup." >&3; exit 1; }
  curl -sf "http://127.0.0.1:${ML_PORT}/health"
}
WAIT_TIMEOUT=$((WAIT_TIMEOUT * 3)) wait_for "ML service" ml_ready
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
log "  ML:        http://127.0.0.1:${ML_PORT}"
log "  Scheduler: PID $SCHEDULER_PID"
log "  Worker:    PID $WORKER_PID"

# bash 3.2 has no `wait -n`, so poll: stop everything as soon as any process exits.
while :; do
  for name_pid in "api:$API_PID" "ml:$ML_PID" "worker:$WORKER_PID" "scheduler:$SCHEDULER_PID"; do
    if ! kill -0 "${name_pid#*:}" 2>/dev/null; then
      log "${name_pid%%:*} exited; stopping."
      exit 1
    fi
  done
  sleep 2
done
