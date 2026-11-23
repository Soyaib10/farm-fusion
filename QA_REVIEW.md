# QA review — Farm Fusion

Reviewed: 2026-11-23. Static read of the redesigned tree. No code was changed. Tests were not executed and the app was not started.

Scope: Go API (`cmd/api`), scheduler, worker, Postgres repositories, migrations, Redis, RabbitMQ, SMTP, OpenWeather client, FastAPI ML service, `docker-compose.yml`, `run.sh`, and `README.md`.

Severity:

- **High** — broken security property, lost or duplicated user-facing alerts, or a production config the repo itself will accept.
- **Medium** — real bug or trust gap that needs a specific condition (race, bad env, second replica, stale cache).
- **Low** — hardening, docs, or hygiene. Safe to schedule after the highs.

## Already solid

These are working as written. Do not rip them out while fixing the items below.

- SQL in the repositories uses placeholders. No string-built queries in Go.
- Passwords are bcrypt. Login runs a dummy compare when the email is missing, so login timing does not reveal accounts. `alg=none` JWTs are rejected. Refresh tokens are stored as SHA-256, not raw.
- Register always sets `role` to `user`. Unknown JSON fields are rejected, so a client cannot send `"role":"admin"` on register.
- Farm and weather handlers pass the caller id into ownership checks. User routes use `CanAccessUser`.
- Production rejects CORS `*`. Security headers and a body size limit exist. OpenWeather transport errors are scrubbed so the API key does not leak in the error string. SMTP subjects strip CR/LF.
- Auth, user, farm, weather, validator, config, and scheduler unit tests assert real behavior (status codes, forbidden, token replay, header injection). They are not empty stubs.

## Fix first

1. Stop treating a signed access JWT as proof the account still exists (F1).
2. Make weather mail idempotent and stop counting a successful SMTP DATA as a failure (F5, F6).
3. Fail closed on `ML_API_KEY` and stop loading pickle in the API process (F4).
4. Tighten production secret and database TLS checks so the sample `.env` cannot boot as production (F2, F3).
5. Make the scheduler and RabbitMQ publisher survive a second run and a broker blip (F7, F8, F9).

---

## High

### F1. Access tokens stay valid after logout and account deletion

`VerifyAccessToken` in `internal/service/auth/token.go` (lines 68–103) checks signature, issuer, audience, expiry, subject, and the `role` claim. `RequireAuth` trusts that result and never loads the user.

`Logout` (`internal/service/auth/service.go` lines 197–206) only revokes the refresh-token hash. `Login` does not revoke older refresh tokens. `UserRepository.Delete` removes the row (refresh rows cascade). The access JWT does not. `ValidateAuth` allows an access lifetime up to 24 hours (`internal/config/config.go` lines 263–264). Default is 15 minutes, which limits the window, but production can set 24 hours and still start.

A logged-out or deleted user, including a deleted admin, can keep calling the API until `exp`. `GET /api/v1/users` authorizes from the token role, so that admin token can still list every user after the account row is gone.

**Fix:** Store a `token_version` (or session id) on the user. Put it in the access token. Reject the request when the user row is missing or the version does not match. Bump the version on delete and on “log out everywhere.” Keep the access TTL short (15 minutes or less). For single-device logout, deny that token’s `jti` until `exp`.

### F2. Production database TLS check allows unverified TLS and plaintext

`Config.Validate` (`internal/config/config.go` lines 239–241) only rejects the substring `sslmode=disable`.

pgx defaults a missing `sslmode` to `prefer`: try TLS with certificate verification skipped, then fall back to plaintext. `sslmode=allow` and `sslmode=prefer` pass this check and can send the database password and refresh-token hashes in the clear. `sslmode=require` also passes, and pgx skips certificate verification for it, so a forged certificate is accepted. `config_test.go` treats `sslmode=require` as the production success case.

**Fix:** Parse the URL. In production, require `sslmode=verify-full` (or `verify-ca` plus an explicit root). Reject missing, `disable`, `allow`, `prefer`, and `require`.

### F3. The sample JWT secret is accepted in production

`ValidateAuth` (`internal/config/config.go` lines 251–262) rejects secrets that contain `change-me` or `your-secret`. The value in `.env.example` (and `README.md` around line 517) is `replace-with-a-secure-random-32-byte-secret-key`. That string is longer than 32 bytes and contains neither phrase. `ENV=production` plus a copied example file starts, and anyone who knows the string can mint an HS256 access token with `"role":"admin"`.

**Fix:** Deny that exact example string, in every environment. Compare against a denylist of the strings shipped in the repo. Add a test that the `.env.example` value fails `ValidateAuth` when `ENV=production`.

### F4. ML predict routes are open when the key is empty, and startup unpickles committed models

`verify_internal_api_key` in `ml_service/app/routers/recommendation.py` (lines 14–24) returns immediately when `ML_API_KEY` is blank. `.env.example` ships it blank. Go `Validate` never requires the key, and `mlclient.postJSON` (`internal/platform/mlclient/client.go` lines 136–138) omits `X-Internal-API-Key` when the key is empty. `run.sh` binds uvicorn to `127.0.0.1`, so this is not remotely reachable in the local script. It becomes an unauthenticated prediction API the moment the process is bound to a non-loopback address, which the code allows.

`ml_service/app/main.py` (lines 32–36) calls `joblib.load` on four committed `.pkl` files at startup, before any request check. Pickle runs code from the file. A tampered `crop_model.pkl`, `fertilizer_model.pkl`, `soil_encoder.pkl`, or `crop_encoder.pkl` executes as the ML process. The parent-directory check only blocks path traversal of the hardcoded names. `.gitignore` does not ignore `*.pkl`. These files are in the tree (crop model about 3.5 MB, fertilizer model about 443 KB).

**Fix:** Refuse to start, in both FastAPI and Go, when `ML_API_KEY` is missing or shorter than 32 bytes. Send the header on every ML request. Stop loading pickle in the service. Export ONNX, or use a format that does not execute code, and verify a checked-in SHA-256 before load. Remove the `.pkl` files from git history if they were ever public, add `ml_service/app/models/*.pkl` to `.gitignore`, and train in CI from the CSVs.

### F5. Weather email is not idempotent, so any redelivery sends it again

`RunScheduled` puts `NotificationID` on the message (`internal/service/notification/scheduler.go` lines 125–126). `Process` (`internal/service/notification/processor.go` lines 42–71) never reads it. Each attempt inserts a new `uuid.New()` row. `notification_log` (`migrations/006_create_notification_log_table.up.sql`) has no unique delivery key.

The worker acks with `_ = delivery.Ack(false)` (`internal/platform/queue/consumer.go` lines 140–144). A failed ack, a crash after SMTP accepts the message, or a broker redelivery sends the mail again.

If the send works and the log insert fails, the error is only logged and `nil` is returned, so the message is acked with no durable record.

`notification_log` uses `ON DELETE CASCADE` from farms and users. Deleting an account removes the only row a later dedupe check could use. The queued message still contains the email address, so mail can go out after the account is gone.

**Fix:** Persist `notification_id` with a unique constraint before sending. On conflict, skip when `email_sent` is already true. Treat a successful SMTP accept as sent even if the follow-up log update fails, and retry only that update. Do not cascade-delete the delivery key you still need for dedupe.

### F6. A failed SMTP QUIT after accepted DATA is counted as “not sent”

`send` in `internal/platform/mailer/smtp.go` (lines 155–159) returns `client.Quit()` after `wc.Close()` succeeded. `Close` waits for the server’s 250, which means the message was accepted. A QUIT error is then a send failure. `Process` sets `EmailSent: sendErr == nil` and the consumer retries (F5), so the user gets a second copy. The log row says `email_sent = false` for a message the mailbox already has.

**Fix:** If `Data` closes successfully, return success. Log a QUIT error and do not fail the delivery.

### F7. A second scheduler, or a second run the same day, publishes the same alerts again

`RunScheduled` (`scheduler.go` lines 76–147) and `cmd/scheduler/main.go` (lines 57–76) have no advisory lock, leader election, or “already published for this farm and window” row. Every matching farm gets a new `NotificationID` and a new publish. A second replica, a `SCHEDULER_RUN_ONCE` process beside the daemon, or a manual rerun sends every alert again.

A failed 05:00 run is not retried until the next day. `run` (`cmd/scheduler/main.go` lines 79–84) logs `ListAll` failure and returns. The loop then sleeps until the next 05:00. Per-farm forecast, user, or publish errors are appended to `result.Errors` and the run is still logged as completed, so those farms are skipped for 24 hours. `SCHEDULER_RUN_ONCE` uses the same helper and then returns, so the process exits 0 even when the run failed. Cron will not retry.

**Fix:** Take `pg_try_advisory_lock` for the whole run. Insert a unique `(farm_id, alert_date)` outbox row before publish, and commit it only after the broker confirm. Retry with backoff, and retry only the farms that failed. On run-once, exit non-zero when `err != nil` or `len(result.Errors) > 0`.

### F8. The RabbitMQ publisher gives up after five reconnects

`watchConnection` in `internal/platform/queue/publisher.go` (lines 93–126) tries five times (backoff about 30 seconds) and then returns. Nothing starts the watcher again. A broker restart longer than that leaves `Publish` failing for the life of the process. The scheduler does not exit, so a supervisor will not restart it.

`connect` (lines 68–69) assigns `p.conn` and `p.ch` without closing the previous connection. The scheduler context is cancelled only on SIGTERM, so a stuck publish has no deadline of its own. The mutex is released before `confirm.WaitContext` (lines 149–154); the hang risk is the missing per-publish timeout, not the lock being held across the confirm.

**Fix:** Reconnect indefinitely with a capped delay and jitter. Close the old connection before replacing it. Bound each publish with its own timeout. Exit non-zero when publishing is dead so the supervisor restarts the process.

### F9. Retry and dead-letter publish is unconfirmed and shares the consume channel

`retryOrDeadLetter` (`internal/platform/queue/consumer.go` lines 150–175) publishes a copy, then the caller acks the original. `PublishWithContext` uses `mandatory=false`, waits for no confirm, and uses the same `c.ch` that `Consume` is delivering on. An AMQP channel is not safe for concurrent publish and consume. Ack and Nack errors are discarded (lines 133–144).

What that does:

- Publish succeeds, ack fails: the original is redelivered and the retry copy is also delivered (F5).
- The broker drops the publish (unroutable, or crash before it is persisted): the original is acked and the alert is gone.
- Publish fails: `Nack(false, true)` requeues immediately with no backoff and no incremented retry count, so a broken channel tight-loops.
- Bad JSON is `Nack(false, false)`, which drops the message. The main queue is declared without a dead-letter exchange.
- Nothing consumes `weather.notifications.dlq`. After the retries the day’s alert sits there. The scheduler will not republish it.

The worker passes the process context into `Process` (`cmd/worker/main.go` lines 47–49). Shutdown cancels that context, the handler returns an error, and `retryOrDeadLetter` publishes with the same cancelled context.

**Fix:** Use a second channel for republish. Enable confirms. Ack the original only after the confirm. On publish failure, dead-letter or leave the message unacked. Dead-letter the main queue. Alert on DLQ depth, or replay it. Give the in-flight message its own timeout, separate from process shutdown.

### F10. `TRUST_PROXY` lets the client choose the rate-limit key

`extractClientIP` (`internal/transport/http/middleware/ratelimit.go` lines 118–131) uses the first `X-Forwarded-For` hop, then `X-Real-IP`. It does not check that `RemoteAddr` is a trusted proxy. Default `TRUST_PROXY` is false (`config.go` line 142). When it is true, which is the usual setting behind a reverse proxy, a client can send a new `X-Forwarded-For` on every login attempt and skip the auth bucket (2 requests/second, burst 5). New keys also grow the in-memory map until the one-minute cleanup.

`NewRateLimiter` rewrites `rps <= 0` to 10 and `burst < 1` to 20. That is looser than the auth defaults of 2 and 5, so a bad env value weakens the limiter while `RATE_LIMIT_ENABLED` stays true.

**Fix:** Ignore forwarding headers unless `RemoteAddr` is in an explicit proxy CIDR list. Use the rightmost address that proxy appended, not the leftmost client-supplied hop. Cap the number of buckets. Reject non-positive limits at startup.

---

## Medium

### F11. Refresh-token reuse detection is not atomic and can fail open

`Refresh` (`internal/service/auth/service.go` lines 163–193) consumes the old token. `issueTokenPair` (lines 208–231) inserts the new one later. Those are separate autocommit statements. On reuse, the revoke-all error is discarded (line 176):

```go
if existing.Revoked {
    _ = s.tokens.RevokeAllForUser(ctx, existing.UserID)
    return nil, domain.ErrTokenRevoked
}
```

If a thief wins `ConsumeActive` and the victim’s `RevokeAllForUser` runs before `Save`, the new refresh row is inserted afterward and stays valid. A database error from `GetByHash` is also turned into `domain.ErrUnauthorized` (lines 170–172), so a lookup outage looks like a bad token. A failed `Save` after a successful consume burns the only refresh token and returns 500.

The sequential replay path is covered by `TestAuthRegisterLoginRotateAndReplayDetection`. The race and the ignored error are not.

**Fix:** Do consume, reuse handling, and insert in one transaction (`SELECT … FOR UPDATE`, or one `UPDATE … RETURNING` plus the insert). If revoke-all fails, return 500 and do not issue a new token. Return 500 for lookup errors that are not `ErrNotFound`.

### F12. Registration reveals which emails exist, and there is no confirmation

`Register` (`service.go` lines 97–104) returns `domain.ErrAlreadyExists` when `GetByEmail` finds a row, and it skips bcrypt in that branch. That maps to HTTP 409. A new email returns 201 and spends time hashing. Login is equalized. Register is not. There is no email verification, so a caller can also hold an address by registering it first. There is no password-reset flow, so a locked-out owner has no recovery path in this codebase.

**Fix:** Always run the same bcrypt work. Return one response for new and existing emails. Send a confirmation link before the account can log in. Add a password reset that does not reveal whether the address exists.

### F13. Email can be changed with only the access token

`user.Service.Update` (`internal/service/user/service.go` lines 45–89) checks `actor.CanAccessUser` and then writes the new email. The handler body has `name` and `email` and no current password. A stolen access token (up to 24 hours, and still valid after logout — F1) can point the account’s email at the attacker. Login is by email, so the owner is locked out. Weather mail follows the new address.

The same method is a lost update. It loads the row, changes name and/or email, and the repository writes both columns. Two concurrent patches (name and email) both start from the same snapshot. The later commit restores the other field. There is no `FOR UPDATE`.

**Fix:** Require the current password for an email change, and confirm the new address before it replaces the old one. Update only the provided column: `SET name = COALESCE($1, name), email = COALESCE($2, email)`, or lock the row inside a transaction.

### F14. Login throttling is per IP and per process

`RateLimiter` keeps buckets in a process-local map keyed by IP. `Login` does not count failures per account. Several API replicas multiply the budget. Many source IPs can try one password across accounts, or many passwords against one account, without a shared counter. This stacks with F10 when `TRUST_PROXY` is on.

**Fix:** Add a per-account failure counter in Postgres or Redis, with a lockout or delay. Share the IP limiter across instances.

### F15. `WEATHER_CACHE_TTL=0` caches forecasts with no expiry

`getEnvInt("WEATHER_CACHE_TTL", 10800)` (`config.go` line 178) accepts `0`. `ForecastCache.Set` (`internal/platform/cache/forecast.go` lines 76–81) passes that duration to Redis. go-redis treats expiration `0` as no expiry, so the first forecast stays forever and later runs alert on stale weather.

A cache hit in `FetchForecast` (`openweather.go` lines 64–71) returns immediately. It does not check that `DataPoints` is non-empty or that `FetchedAt` is inside the TTL. A 200 with an empty `list` is stored and then served as a hit, which suppresses alerts until the key is deleted.

`NewForecastCache` replaces the whole `redis.Options` when `REDIS_URL` is set, so `REDIS_PASSWORD`, `REDIS_ADDR`, and `REDIS_DB` on the side are dropped.

**Fix:** Reject TTL `<= 0`. On a hit, require non-empty points and `FetchedAt` within TTL. Otherwise delete the key and refetch. When the URL is set, copy password, DB, and timeouts onto the parsed options.

### F16. Weather operator `=` uses exact float equality

`compare` (`scheduler.go` lines 256–257) uses `actual == expected` for `=` and `==`. Forecast values are JSON floats. `30.5` can arrive as `30.499999999`. An equality rule the API allows will almost never match, so that alert never sends. `<`, `<=`, `>`, `>=` are fine.

**Fix:** Compare with a small epsilon, or reject `=` at validation time.

### F17. Daily schedule drifts across DST, and alert times have no zone

`nextDailyRun` (`cmd/scheduler/main.go` lines 99–104) adds `24 * time.Hour` when today’s 05:00 is not strictly after `now`. That is not a calendar day. Around a DST change the next run lands at 04:00 or 06:00 local. If `now` is exactly 05:00:00, `!next.After(now)` is true, so that day’s run is skipped and the next one is 24 hours later.

Forecast instants are stored with `time.Unix(...).UTC()` (`openweather.go` line 114). The email prints ranges without a zone name. The process zone is the container’s zone. The schema has no per-farm timezone.

**Fix:** Step with `AddDate(0, 0, 1)` in a configured location, and treat equal-to-05:00 as due. Print the zone on every timestamp.

### F18. Scheduler loads every farm, then two more queries per farm

`ListAll` is an unbounded `SELECT` from `farms`. For each farm, `RunScheduled` calls `ListByFarmID` and `GetByID` (lines 85–123), including farms with no enabled rules. `ListByFarmID` does not filter `is_enabled = true`. There is no statement timeout on the Postgres pool.

**Fix:** One query that joins farms, enabled alerts, and users, paged by `location_key`. Set `statement_timeout` on that session.

### F19. Foreign-key failures are reported as “not found”

`mapError` (`internal/repository/postgres/errors.go` lines 29–30) maps SQLSTATE `23503` to `domain.ErrNotFound`. Creating a farm for a missing user, or a weather rule for a deleted farm, looks the same as a missing row. Check failures (`23514`, the coordinate, operator, and role checks from migration 007) are not mapped, so they surface as a wrapped driver error.

**Fix:** Map `23503` on insert to an invalid-reference error. Map `23514` to a validation error. Keep `23503` as not-found only where a missing parent should look like a missing resource.

### F20. ML client hides cancellation and turns 4xx into “upstream unavailable”

`postJSON` (`internal/platform/mlclient/client.go` lines 140–150) logs the transport error and returns `ErrUpstreamUnavailable` without wrapping the original error. `errors.Is(err, context.Canceled)` is false. Every non-200, including 400 and 422, becomes the same upstream error. The body is not drained before `Close` on the error path, so the connection is not reused.

**Fix:** Wrap the transport error with `%w`. Map 4xx to a validation or contract error and 5xx or network errors to `ErrUpstreamUnavailable`. Drain the body on non-200.

### F21. Fertilizer API bounds do not match the training data

Checked against `ml_service/data/fertilizer_recommendation.csv` (99 rows):

| Field | CSV | API (`recommendation.py` and the Go service) |
| --- | --- | --- |
| Nitrogen | 4–42 | 0–39 |
| Soil moisture | 25–65 | 10–65 |
| Temperature, humidity, potassium, phosphorous | match the CSV min/max | match |

Callers cannot submit the high-nitrogen region the model was trained on (several rows are 40–42 and labeled Urea, including line 9). Soil moisture `ge=10` accepts values the model never saw. Label counts are uneven: `17-17-17` and `10-26-26` have 7 rows each.

**Fix:** Set nitrogen `le=42` and soil moisture `ge=25`, or retrain on a wider set and then widen the API. Derive Go and Pydantic limits from one shared spec.

### F22. Training leaks encoder state into the holdout and has no accuracy gate

`train_fertilizer_model` (`ml_service/train_models.py` lines 41–63) `fit_transform`s both `LabelEncoder`s on the full 99-row frame, then splits. The test set affects the codes used to score and to save the encoder. The split is unstratified. With 7 rows in two classes, a 20% holdout can drop a class from train, and the saved model can never predict it.

There is no required-column check and no minimum accuracy. A bad score still writes the pickle. Line 81 prints success whenever nothing threw. Crop training does not put the label into `X`. It also does not set `max_depth` and does not scale features.

`run.sh` (line 73) retrains only when `crop_model.pkl` is missing. Those pickles are already in git, so a clone never runs `train_models.py`. If crop training saved and fertilizer training then failed, later startups skip training and FastAPI dies in lifespan on the missing files.

**Fix:** Validate columns. `fit` encoders on the training fold only. Use `stratify=y`. Refuse to write artifacts below a recorded threshold. Refit the final model on all rows only after the metric is logged. In `run.sh`, require all four artifacts or a checksum, and delete a partial `app/models/` directory on failure.

### F23. Python dependencies and images are not pinned

`ml_service/requirements.txt` uses ranges (`scikit-learn>=1.5.0,<2.0.0` and the same pattern for the rest). There is no lockfile. A later install can load a different scikit-learn than the one that pickled the committed models, which breaks `joblib.load` or changes predictions. Compose tags (`postgres:16-alpine`, `redis:7-alpine`, `rabbitmq:3.13-management-alpine`) are not digest-pinned. `go.mod` / `go.sum` are fine.

**Fix:** Commit a hashed lock (`pip-compile --generate-hashes` or `uv lock`) and install with `--require-hashes`. Pin the three images by digest.

### F24. README states a training pipeline and accuracy the code does not produce

`README.md` (lines 21, 147–150, 209–215) says crop accuracy is 99.32%, fertilizer is about 95%, inference is under 50 ms, models are about 2 MB, features are normalized, trees have a max depth, and Decision Tree, SVM, KNN, and logistic regression were compared. `train_models.py` only prints one unstratified holdout score. It sets `n_estimators=100` and `random_state=42` only. The committed crop pickle is about 3.5 MB and the fertilizer pickle about 443 KB. Line 647 points at an MIT `LICENSE` file. That file is not in the tree.

The 05:00 schedule, the 2,200-row crop CSV, and `static/farm-fusion.png` are real.

**Fix:** Replace the performance block with numbers the training script writes (accuracy, per-class support, model hashes, byte sizes). Delete the model-bake-off and normalize/max-depth claims, or add the code that does them. Add a `LICENSE` or drop the MIT sentence.

### F25. Farm and weather reads distinguish “missing” from “not yours”

`farmsvc.Service.GetByID` (`internal/service/farm/service.go` lines 82–90) returns `ErrNotFound` when the id is missing and `ErrForbidden` when the farm exists but belongs to someone else. Weather methods return that same error. Those become HTTP 404 and 403. Any logged-in caller can probe farm UUIDs. User routes do not have this split: `GetByID` returns `ErrForbidden` before the lookup.

**Fix:** Return 404 for both “missing” and “not yours” on farm and weather reads and writes.

### F26. Authenticated profile and farm responses can be cached

`NoStore` is applied only under `/api/v1/auth` (`internal/transport/http/router.go` lines 57–61). `GET /api/v1/users/me`, `GET /api/v1/users/{id}`, and `GET /api/v1/farms` return email and coordinates with no `Cache-Control`.

**Fix:** Set `Cache-Control: no-store` on every authenticated route.

---

## Low

### F27. Local Compose defaults are weak, and the ML service is not in Compose

`docker-compose.yml` defaults Postgres to `postgres` / `password` (lines 6–7) and RabbitMQ to `guest` / `guest` (lines 34–35). Redis has no password. Ports are bound to `127.0.0.1`, which keeps this off the LAN. Nothing sets `user:`, `cap_drop`, or `no-new-privileges`. Healthchecks exist for the three infra services. The ML process is not in Compose. `Config.Load` defaults `RABBITMQ_URL` to `amqp://guest:guest@127.0.0.1:5672/`. Empty `SERVER_HOST` listens on all interfaces. Production config does not require a reverse proxy or TLS in front of the API. `REFRESH_TOKEN_DURATION` only has to be positive, so a multi-year refresh token is accepted, and login does not clear older sessions.

**Fix:** Require `POSTGRES_PASSWORD`, `RABBITMQ_DEFAULT_PASS`, and `REDIS_PASSWORD` with no weak defaults. Add `requirepass` and a healthcheck that uses it. Drop privileges. Put the ML service in Compose with a `/health` check and a non-root user. Fail production startup when the RabbitMQ URL still contains `guest`. Cap refresh lifetime (for example 30 days) and add an explicit revoke-all.

### F28. `run.sh` can hang, and migration names are interpolated into SQL

The `pg_isready`, `redis-cli`, and `nc` loops (`run.sh` lines 39–43) have no deadline. `version="$(basename "$f")"` is spliced into `WHERE version = '$version'` and `INSERT ... VALUES ('$version')` (lines 57–61). A migration filename with a quote breaks out of that string. After startup, `wait` follows only the API pid (line 113), so a dead ML process leaves the script running. `nc` always checks port 5672, not `RABBITMQ_PORT`.

**Fix:** Timeout each wait. Apply the version with `psql -v version=...` and a quoted `:'version'` literal, or reject filenames that are not `^[0-9]+_.+\.up\.sql$`. `wait` on the ML pid as well. Probe RabbitMQ on the configured port.

### F29. Production still serves `/openapi.json`

`ml_service/app/main.py` (lines 43–47) disables `docs_url` when `ENV=production` and always disables `redoc_url`. `openapi_url` is left at the FastAPI default `/openapi.json`, so the schema stays public with no API key.

**Fix:** Set `openapi_url=None` whenever docs are disabled.

### F30. Secret and model ignore rules are incomplete

`.gitignore` ignores `.env` and `.env.local`. It does not ignore `.env.production` or `.env.staging` (`.env.*.local` does not match them). Pickle artifacts are not ignored, which is how the four model files stayed tracked.

**Fix:** Ignore `.env.*` and keep `!.env.example`. Ignore `*.pkl` and `ml_service/app/models/`.

### F31. Password minimum length counts bytes, not characters

`StrongPassword` uses `len(pw) < 8`. The error text says “8 characters.” Three CJK letters plus a digit is 4 characters and more than 8 bytes, so it passes. The 72-byte bcrypt cap (`len(pw) > 72`) is the right check and should stay.

**Fix:** Require `utf8.RuneCountInString(pw) >= 8` and keep the 72-byte maximum.

### F32. Migration 007 unique index can fail on existing duplicates

`migrations/007_add_role_and_security_constraints.up.sql` creates a unique index on `refresh_tokens (token_hash)`. Migration 002 only added a non-unique index. The migration fails if any duplicate hashes already exist. `UNIQUE (LOWER(email))` fails the same way if mixed-case duplicate emails were inserted outside the app.

**Fix:** Delete or merge duplicate hashes and case-variant emails in the same migration before creating the unique indexes. This only matters for databases that already have rows. A fresh database is fine.

### F33. `retryCount` drops some AMQP integer types

`retryCount` in `internal/platform/queue/topology.go` handles `int`, `int32`, `int64`, and `float64`. This client publishes `int32`. The AMQP library can also decode an integer header as `uint32`. That type hits `default` and returns 0, so the counter resets and the message retries forever instead of going to the DLQ. This only happens if a hop rewrites the header type.

**Fix:** Also accept `uint32`, `int16`, `int8`, and numeric strings. Log and dead-letter on an unknown type.

### F34. Migration 005 backfill can disagree with `NewLocationKey`

The SQL backfill is `ROUND(latitude::numeric, 2)::text`. Go uses `math.Round(value*100) / 100` and `%.2f`. Those differ on halfway float values. `1.005` in Go is `1.004999999999999893` and becomes `1.00`. PostgreSQL `float8::numeric` can round that same bit pattern back to `1.005`, then `ROUND(..., 2)` yields `1.01`.

The scheduler uses the stored `location_key` when it is non-empty (`scheduler.go` lines 92–95). A backfilled farm and a farm created later by Go at the same coordinate may not share a cache entry. Forecasts are still fetched for each farm’s real lat/lon. New writes from `farm.Service` use Go only, so they agree with each other. This was not executed against a live Postgres.

**Fix:** Backfill with the same function the app uses, or recompute keys in Go once and `UPDATE` them.

---

## Tests and CI

Nothing runs tests on push. There is no `.github/` workflow. `make test` is `go test ./...` and does not cover Python.

Existing Go tests that do exist are real. They do not hit `/recommend`, most farm routes, or weather routes in `router_test.go`. The recommendation test stubs the predictor.

No tests at all for:

- `ml_service/` (no `test_*.py`)
- `internal/platform/mlclient`, `queue`, `cache`, `database`
- `internal/repository/postgres`
- `internal/service/notification/processor.go` (the scheduler is tested)
- `cmd/api`, `cmd/scheduler`, `cmd/worker` (`nextDailyRun` is untested)
- HTTP handlers and most middleware beyond what `router_test.go` hits

`go test ./...` never loads a pickle, never hits FastAPI, and never opens Postgres, Redis, or RabbitMQ.

**Fix:** CI that runs `go test -race ./...`, `go vet`, a locked `pip install`, and pytest for the ML service. Add tests for F5 (same `notification_id` does not send twice), F6 (QUIT error after DATA is success), F1 (deleted user token is rejected), and F3 (example JWT secret fails production validation).

## Product gaps (not defects in the current code)

- No password reset and no email verification (see F12 and F13).
- No metrics or tracing. Logs are structured and redact secret-like keys. There is no alert when the DLQ grows or the scheduler publishes zero farms.
- User list is unbounded. Fine for a handful of accounts. Add a limit before an admin endpoint is exposed to a large table.
- This tree is the API, worker, scheduler, and ML service. There is no web client in the repo.
