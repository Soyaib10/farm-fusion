# QA solution status — Farm Fusion

Status of every finding in `QA_REVIEW.md` (F1–F34) and its Tests and CI section, as of 2026-11-23.

## Verification run

- `gofmt -l .`, `go build ./...`, and `go vet ./...` are clean.
- `go test -race ./...` passes in every package.
- `pytest` in `ml_service/` passes (16 tests).
- `python train_models.py` passes both accuracy gates.
- `docker compose config` passes with `.env.example` values, and fails as intended when passwords are missing.
- `bash -n run.sh` passes.

Not run: anything against a live Postgres, Redis, RabbitMQ, or SMTP server. The Docker daemon was off, so no container was started. CI has not run because nothing is pushed or committed.

## Categories

| Category | Meaning | Count |
| --- | --- | --- |
| Done | Fixed in code and covered by passing unit tests or static checks | 19 |
| Action needed | Fixed in code, but the fix depends on SQL, a broker, or a script that has not run for real, or the user must act before deploying | 9 |
| Partly fixed | Part of the requested fix is in place and part was deliberately left out | 6 |
| Skipped | No part done | 0 |

The Tests and CI section is tracked separately at the end. It is done, but CI has not run yet.

## Summary table

| ID | Title | Status |
| --- | --- | --- |
| F1 | Access tokens stay valid after logout and account deletion | Done |
| F2 | Production database TLS check allows unverified TLS and plaintext | Done |
| F3 | The sample JWT secret is accepted in production | Done |
| F4 | ML routes open with empty key; startup unpickles committed models | Partly fixed |
| F5 | Weather email is not idempotent | Action needed |
| F6 | Failed SMTP QUIT after accepted DATA counted as not sent | Done |
| F7 | Second scheduler or second run publishes the same alerts again | Action needed |
| F8 | RabbitMQ publisher gives up after five reconnects | Action needed |
| F9 | Retry and dead-letter publish is unconfirmed and shares the consume channel | Action needed |
| F10 | `TRUST_PROXY` lets the client choose the rate-limit key | Done |
| F11 | Refresh-token reuse detection is not atomic | Action needed |
| F12 | Registration reveals which emails exist; no confirmation | Partly fixed |
| F13 | Email can be changed with only the access token | Partly fixed |
| F14 | Login throttling is per IP and per process | Partly fixed |
| F15 | `WEATHER_CACHE_TTL=0` caches forecasts with no expiry | Done |
| F16 | Weather operator `=` uses exact float equality | Done |
| F17 | Daily schedule drifts across DST; alert times have no zone | Done |
| F18 | Scheduler loads every farm, then two queries per farm | Action needed |
| F19 | Foreign-key failures reported as not found | Done |
| F20 | ML client hides cancellation; 4xx becomes upstream unavailable | Done |
| F21 | Fertilizer API bounds do not match training data | Done |
| F22 | Training leaks encoder state; no accuracy gate | Done |
| F23 | Python dependencies and images are not pinned | Done |
| F24 | README states a pipeline and accuracy the code does not produce | Done |
| F25 | Farm and weather reads distinguish missing from not yours | Done |
| F26 | Authenticated responses can be cached | Done |
| F27 | Weak Compose defaults; ML service not in Compose | Partly fixed |
| F28 | `run.sh` can hang; migration names interpolated into SQL | Action needed |
| F29 | Production still serves `/openapi.json` | Done |
| F30 | Secret and model ignore rules are incomplete | Partly fixed |
| F31 | Password minimum length counts bytes | Done |
| F32 | Migration 007 unique index can fail on duplicates | Action needed |
| F33 | `retryCount` drops some AMQP integer types | Done |
| F34 | Migration 005 backfill can disagree with `NewLocationKey` | Action needed |
| Tests/CI | No CI; missing tests | Done (CI not yet run) |

---

## High

### F1. Access tokens stay valid after logout and account deletion — Done

- `Authenticate` loads session state from Postgres on every request: role, `token_version`, and whether the token's `jti` is in `revoked_access_tokens` (migration 008). The role now comes from the database, not the JWT.
- `Logout` also revokes the access token's `jti` when sent as `Authorization: Bearer`. New `POST /api/v1/auth/logout-all` bumps `token_version`, which invalidates every outstanding access token, and revokes all refresh tokens.
- Access tokens are capped at 15 minutes and refresh tokens at 30 days.
- Tests: deleted user is rejected, role comes from the database, logout and logout-all, and a router test for a deleted admin getting 401.
- Not run against real Postgres: `SessionRepository` SQL.

### F2. Production database TLS check allows unverified TLS and plaintext — Done

- In production, `Validate` requires `sslmode=verify-full`, or `verify-ca` with `sslrootcert`. It handles URL and key=value DSNs and falls back to `PGSSLMODE`.
- Tests are in `internal/config/config_test.go`.

### F3. The sample JWT secret is accepted in production — Done

- The `.env.example` JWT and ML key placeholders are rejected in every environment. Any value containing `replace-with` is rejected in production.
- `TestEnvExampleSecretsAreRejected` parses the real `.env.example`.
- **Action needed:** generate real `JWT_SECRET` and `ML_API_KEY` values (`openssl rand -hex 32`), because the placeholders no longer start.

### F4. ML routes open with empty key; startup unpickles committed models — Partly fixed

Done:
- FastAPI (`app/security.py`) and Go (`ValidateML`) refuse to start unless `ML_API_KEY` is at least 32 bytes and is not the placeholder.
- The Go client sends the header on every request.
- The service no longer loads any pickle. It trains both models in-process from the CSVs at startup (`app/training.py`). This replaces the suggested ONNX and checksum approach.
- The four `.pkl` files are untracked (`git rm --cached`) and ignored.
- Tests cover a missing, short, or placeholder key at startup, and a missing or wrong key returning 401.

Not done:
- The `.pkl` files are still in git history. **Action needed:** rewrite history (for example with `git filter-repo`) if the repo was ever public. I did not rewrite it because that is destructive and changes shared history.
- The local copies in `ml_service/app/models/` were left on disk. They are now ignored and unused.

### F5. Weather email is not idempotent — Action needed

- Migration 009 adds a unique `notification_log.notification_id`.
- `Processor.Process` calls `Begin`, which inserts or reuses the row by `notification_id` and skips the email if `email_sent` is already true. It then sends and calls `MarkSent`, which is retried up to 3 times. A failed send calls `MarkFailed` and returns the error so the message is retried.
- The processor reloads the recipient and uses the current email. A deleted user is skipped.
- Tests: the same `notification_id` sends once, retry after a failure, and a deleted recipient is skipped. These use in-memory fakes.
- **Action needed:** run migration 009 and the `NotificationLogRepository` SQL against a real Postgres.

### F6. Failed SMTP QUIT after accepted DATA counted as not sent — Done

- `Sender.Send` returns success once DATA is accepted. A later QUIT error goes to an `onQuitError` callback, which the worker logs.
- Test: `TestSendSucceedsWhenQuitFailsAfterAcceptedData` uses a fake SMTP server.

### F7. Second scheduler or second run publishes the same alerts again — Action needed

- `notification_outbox` (migration 009) has one row per `(farm_id, alert_date)`. The scheduler reserves the row before publishing, reuses its `notification_id`, and marks it published afterwards. A rerun skips farms that are already published.
- `pg_try_advisory_lock` lets only one scheduler run at a time. A second instance logs and skips.
- A failed run retries after 1, 5, and 15 minutes. `SCHEDULER_RUN_ONCE=true` exits non-zero on any error or per-farm failure.
- Test: a second run does not republish (in-memory fakes).
- **Action needed:** run the outbox and advisory-lock SQL against a real Postgres.

### F8. RabbitMQ publisher gives up after five reconnects — Action needed

- The publisher reconnects forever with exponential backoff capped at 30s, with jitter. It closes the old connection first and watches both connection and channel close.
- Each publish has a 15s timeout.
- `Failed()` fires once an outage passes 10 minutes. The scheduler exits on it so a supervisor can restart it.
- **Action needed:** test against a real broker by stopping and starting RabbitMQ while the scheduler runs. No unit test covers reconnects.

### F9. Retry and dead-letter publish is unconfirmed and shares the consume channel — Action needed

- The consumer republishes retries on a separate channel with confirms. It uses `mandatory=true` and checks for returned (unroutable) messages. It acks the original only after the copy is confirmed. Otherwise it nacks without requeue, so the message dead-letters.
- The main queue now has dead-letter arguments pointing at the DLQ.
- Each message gets its own 2-minute handler timeout.
- DLQ depth is logged every 5 minutes.
- `retryCount` tests are in `topology_test.go`.
- **Action needed:**
  - Drain and delete the existing `weather.notifications` queue before deploying. RabbitMQ will not redeclare a queue with new arguments. This is documented in the README.
  - Run against a real broker. The consume, republish, and confirm path has no integration test.

### F10. `TRUST_PROXY` lets the client choose the rate-limit key — Done

- `X-Forwarded-For` is only used when the direct peer is in `TRUSTED_PROXY_CIDRS`. The header is read from the right, skipping trusted hops.
- `TRUST_PROXY=true` without CIDRs fails validation.
- The bucket map is capped at 100k entries, with an overflow bucket.
- Tests are in `middleware/ratelimit_test.go`.

### F11. Refresh-token reuse detection is not atomic — Action needed

- `RefreshTokenRepository.Rotate` runs in one transaction using `SELECT ... FOR UPDATE`. Reusing a revoked token revokes all of the user's sessions and commits, returning `ErrTokenRevoked`. An expired token returns `ErrTokenExpired`.
- The service tests use in-memory stores.
- **Action needed:** run `Rotate` against a real Postgres, including two concurrent refreshes of the same token.

---

## Medium

### F12. Registration reveals which emails exist; no confirmation — Partly fixed

Done:
- `Register` always runs bcrypt, hides "already exists", and always returns `202` with the same message.

Skipped, with reason:
- Email confirmation before login, and a password-reset flow. Both need new token tables, email templates, endpoints, and a product decision on expiry and UX. That is a feature, not a fix. Listed as future work in the README.

### F13. Email can be changed with only the access token — Partly fixed

Done:
- Changing email requires `current_password`, which is checked against the caller's password hash.
- The lost update is fixed. `UpdateProfile` uses `COALESCE`, so only provided fields change.
- Access tokens are revocable (F1).
- There is a test for changing email.

Skipped, with reason:
- Confirming the new address before it replaces the old one. It needs the same verification-token flow as F12.

### F14. Login throttling is per IP and per process — Partly fixed

Done:
- Per-account counter in Postgres (`failed_login_count`, `locked_until`). 5 failures lock the account for 15 minutes. A successful login resets the counter.
- Tested in the auth service tests.

Skipped, with reason:
- A shared IP limiter across API replicas. It needs a Redis-backed limiter and a decision on its behaviour when Redis is down. The README says to put a shared limiter in front when running several replicas.

Known trade-off:
- The lockout lets anyone lock a known email for 15 minutes. That is the usual cost of account lockout.

### F15. `WEATHER_CACHE_TTL=0` caches forecasts with no expiry — Done

- Config rejects a TTL of 0 or less. `NewForecastCache` also rejects a non-positive TTL.
- The cache deletes empty or stale entries and refuses to store an empty forecast.
- The OpenWeather client ignores an empty cached forecast.
- Tests are in `weatherapi`.

### F16. Weather operator `=` uses exact float equality — Done

- Equality uses a relative tolerance of 1e-6.
- Tested in `scheduler_test.go`.

### F17. Daily schedule drifts across DST; alert times have no zone — Done

- `nextDailyRun` steps calendar days with `time.Date` in `SCHEDULER_TIMEZONE`. Exactly the run hour counts as due.
- Alert ranges are shown in the configured zone, and the email template prints the zone.
- Test: `cmd/scheduler/main_test.go` covers the spring-forward and fall-back days in New York.

### F18. Scheduler loads every farm, then two queries per farm — Action needed

- `ListAll` was replaced by `ListAlertTargets`. It reads pages of 500 using a keyset cursor `(location_key, id)` and returns farm, owner email, and enabled alerts in one query.
- **Action needed:** run the CTE query against a real Postgres. Unit tests use a fake reader.

### F19. Foreign-key failures reported as not found — Done

- Postgres error 23503 maps to `ErrInvalidReference`, which returns HTTP 422.
- 23514 maps to a field-level validation error.
- The weather service maps an invalid farm reference to not found, which matches F25.

### F20. ML client hides cancellation; 4xx becomes upstream unavailable — Done

- Transport errors are wrapped with `%w`, so `errors.Is(err, context.Canceled)` works.
- 4xx (except 401 and 403) maps to a validation error. Everything else maps to `ErrUpstreamUnavailable`.
- The body is drained on errors.
- Tests are in `mlclient/client_test.go`.

### F21. Fertilizer API bounds do not match training data — Done

- `ml_service/feature_bounds.json` is the single spec. It sets nitrogen to 0–42 and soil moisture to 25–65.
- Pydantic models are built from it.
- The Go test `TestBoundsMatchMLSpec` fails if the Go bounds drift from it.
- The Python test `test_bounds_cover_training_data` checks the bounds against the CSVs.

### F22. Training leaks encoder state; no accuracy gate — Done

- Required columns and empty values are checked.
- The encoder sits inside a scikit-learn `Pipeline`, so each fold fits it on its own training rows only.
- Scoring uses stratified 5-fold cross-validation. Training rejects any class with fewer rows than folds.
- Accuracy gates are 0.97 for crop and 0.90 for fertilizer. Training refuses below them, then refits on all rows.
- `train_models.py` writes `ml_service/metrics/model_report.json`.
- Measured accuracy: crop 0.9955, fertilizer 0.98.
- The `run.sh` pickle check was removed because there are no artifacts any more.

### F23. Python dependencies and images are not pinned — Done

- `requirements.txt` and `requirements-dev.txt` are hashed locks from `pip-compile`, built from the `.in` files.
- CI, `run.sh`, and the Dockerfile install with `--require-hashes`.
- The Postgres, Redis, RabbitMQ, and `python:3.14-slim` images are pinned by digest.
- The lock was resolved for Python 3.14 only.

### F24. README states a pipeline and accuracy the code does not produce — Done

- The performance block now uses the measured numbers and class counts from the report.
- The model bake-off, normalization, max-depth, latency, and model-size claims were removed.
- The MIT `LICENSE` line was dropped because no `LICENSE` file exists. **Action needed (optional):** add a `LICENSE` file if you want one.
- The new endpoints and behaviour are documented: register returns 202, `logout-all`, `current_password`, delivery guarantees, and the queue redeclare step.

### F25. Farm and weather reads distinguish missing from not yours — Done

- The farm and weather services return `ErrNotFound` (HTTP 404) for both cases.
- Tests were updated.

### F26. Authenticated responses can be cached — Done

- `NoStore` applies to all of `/api/v1`.
- Router test: `/users/me` returns `Cache-Control: no-store`.

---

## Low

### F27. Weak Compose defaults; ML service not in Compose — Partly fixed

Done:
- Compose requires `POSTGRES_*`, `RABBITMQ_USER` and `RABBITMQ_PASS`, `REDIS_PASSWORD`, and `ML_API_KEY` via `${VAR:?}`.
- Redis runs with `requirepass`, and its healthcheck authenticates via `REDISCLI_AUTH`.
- All services use `no-new-privileges` and `cap_drop: ALL`.
- The ML service is under the `ml` profile, with a non-root Dockerfile, a read-only root filesystem, and a `/health` check.
- Production rejects a RabbitMQ `guest` URL.
- `SERVER_HOST` defaults to `127.0.0.1`.
- Refresh tokens are capped at 30 days, and `logout-all` exists.

Skipped, with reason:
- Requiring a reverse proxy or TLS in front of the API in production. The app cannot reliably detect that, and it is a deployment decision.

Action needed:
- Start the stack with Docker. The infrastructure containers add back only `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, and `SETUID`, and that has not been tried. If a container fails to start, it probably needs one more capability.
- Build the ML image. It has not been built.

### F28. `run.sh` can hang; migration names interpolated into SQL — Action needed

- Every wait has a deadline. `WAIT_TIMEOUT` defaults to 60s, and the ML wait gets three times that.
- Migration filenames must match `^[0-9]+_[A-Za-z0-9_]+\.up\.sql$`. The version is passed with `psql -v version=...` and used as `:'version'` through stdin.
- Each migration and its version row are applied in one transaction.
- RabbitMQ is probed on `RABBITMQ_PORT`. Redis is probed with its password.
- Only the infrastructure services are started.
- A polling loop compatible with bash 3.2 stops everything if any of the API, ML, worker, or scheduler processes exits.
- **Action needed:** run `./run.sh` end to end. Only `bash -n` and the filename regex were checked.

### F29. Production still serves `/openapi.json` — Done

- `openapi_url` is `None` whenever docs are off (`ENV=production`).

### F30. Secret and model ignore rules are incomplete — Partly fixed

Done:
- `.gitignore` adds `.env.*`, `!.env.example`, `*.pkl`, `*.joblib`, and `ml_service/app/models/`. Checked with `git check-ignore`.
- The pickles were removed from the index.

Not done:
- Removing the pickles from git history. That is the same action-needed item as F4.

### F31. Password minimum length counts bytes — Done

- Uses `utf8.RuneCountInString(pw) >= 8` and keeps the 72-byte maximum.
- There is a test with CJK input.

### F32. Migration 007 unique index can fail on duplicates — Action needed

- Before the unique indexes, the migration:
  - deletes duplicate refresh-token hashes;
  - merges case-variant emails into the oldest account, moving farms and `notification_log` rows;
  - lowercases emails.
- **Action needed:** run it against a copy of a real database that has duplicates. It was never executed. A database that already applied 007 will not rerun it.

### F33. `retryCount` drops some AMQP integer types — Done

- Accepts all signed and unsigned integer types, floats, and numeric strings.
- An unknown type returns `ok=false`, and the message is dead-lettered.
- Tests are in `topology_test.go`.

### F34. Migration 005 backfill can disagree with `NewLocationKey` — Action needed

- The scheduler runs `ReconcileLocationKeys` at startup, which recomputes keys in Go and updates rows that differ.
- The scheduler also computes the key in Go for each target instead of trusting the stored value.
- **Action needed:** run the reconcile SQL against a real Postgres.

---

## Tests and CI — Done (CI not yet run)

- `.github/workflows/ci.yml` runs:
  - Go: `gofmt`, `go vet`, and `go test -race ./...`.
  - ML: `pip install --require-hashes`, `train_models.py` (accuracy gate), and `pytest`.
- `make test` runs both the Go race tests and pytest.
- Tests the review asked for:
  - F5: `processor_test.go`, same `notification_id` sends once.
  - F6: `smtp_test.go`, QUIT error after DATA counts as success.
  - F1: auth service tests and `router_test.go`, a deleted user's token is rejected.
  - F3: `config_test.go`, the example secret fails validation.
- Also added tests for `mlclient`, queue `retryCount`, the rate limiter, the processor, `nextDailyRun`, OpenWeather caching, and the ML service (API and training).

Still missing:
- No tests against Postgres, Redis, RabbitMQ, or SMTP: repositories, migrations, cache, and consumer or publisher integration. This needs containers in CI, for example testcontainers or service containers.

**Action needed:** push a branch and confirm the workflow passes on GitHub. The Python lock was resolved on macOS for Python 3.14, and the Linux runner has not installed it yet.

---

## Before deploying

1. Set real `JWT_SECRET`, `ML_API_KEY`, `POSTGRES_PASSWORD`, `RABBITMQ_PASS`, and `REDIS_PASSWORD` in `.env`.
2. Drain and delete the old `weather.notifications` RabbitMQ queue.
3. Apply migrations 008 and 009. Run 007 on a database copy first if it has never been applied.
4. Run `./run.sh` and send one test alert end to end.
5. Decide whether to rewrite git history to purge the `.pkl` files.
6. Push and confirm CI passes.
