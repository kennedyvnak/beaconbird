# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

BeaconBird is a multi-channel notification delivery system in Go. It handles ingestion, scheduling, retry, and delivery of notifications across email (AWS SES) and push (FCM) channels.

Two binaries:
- `cmd/api` — HTTP API (chi router)
- `cmd/worker` — poll loop + scheduler + heartbeat

## Architecture

Dependencies point inward. No central interface registry.

```
cmd → api/worker → service → repository/provider (concrete)
```

- `internal/domain` — pure Go types + status consts. No json/db/sqlc tags.
- `internal/repository` — concrete postgres implementations wrapping sqlc
- `internal/provider` — concrete SES, FCM implementations
- `internal/service` — business logic (only where logic actually exists)
- `internal/api` — chi HTTP handlers, DTOs, middleware
- `internal/worker` — poller, executor, heartbeat, scheduler

**Interface rules**:
- Interfaces defined in the package that consumes them, not in a central package
- `service/` declares the repo/provider interfaces it needs inline
- `worker/` declares its own repo interfaces inline
- Handlers may call repos directly via inline interface if no service logic needed

**Rule**: domain imports nothing internal. sqlc structs mapped to domain types at repo boundary.

## Tech stack

- Go 1.22+, PostgreSQL 16 with `pgx/v5`
- `sqlc` for type-safe query generation
- `golang-migrate` for migrations (file-based SQL)
- `chi` for HTTP routing
- AWS SES via `aws-sdk-go-v2/service/sesv2`
- FCM via `firebase.google.com/go/v4`

## Makefile targets

```
make run-api          go run cmd/api/main.go
make run-worker       go run cmd/worker/main.go
make migrate-up       apply all migrations
make migrate-down     roll back one step
make migrate-create   create new migration pair (name=...)
make sqlc             regenerate from db/queries/
make test             go test ./...
make lint             golangci-lint run
make build            build both binaries to bin/
make docker-up        docker compose up -d
```

## Database

Migrations in `db/migrations/` — always write both `.up.sql` and `.down.sql`.

Query files in `db/queries/*.sql`. Generated code goes to `internal/repository/sqlc/` — never edit generated files by hand.

## Key domain rules

### Notification lifecycle

1. POST /v1/notifications received
2. Idempotency check against `notifications.idempotency_key`
3. One `notification` row created (status `pending`), one `delivery_job` per entry in `deliveries[]`
4. If `send_at == "now"`, jobs immediately eligible for polling
5. Worker polls `delivery_jobs WHERE status IN ('pending','retrying') AND send_at <= now`
6. Worker locks row with `locked_at` + `locked_by` (worker instance UUID)
7. On success: status → `sent`, attempt logged
8. On failure: retry_count++, status → `retrying`, next_retry_at set (exponential backoff)
9. On retry_count >= max_retries: status → `dead_letter`

### Delivery job status machine

```
pending → processing → sent
                    ↘ failed → retrying → processing (next attempt)
                                        ↘ dead_letter
pending → cancelled
```

### send_at handling

- `"now"` → store as `NOW()` UTC
- ISO 8601 string → parse, normalize to UTC, reject if in the past by > 30s
- All times stored as `TIMESTAMPTZ`

### Retry backoff

```
wait = base_delay * 2^retry_count + jitter
base_delay=30s, max_delay=1h, jitter=rand(0..10s)
```

### Worker recovery

Goroutine runs every 60s: finds jobs where `locked_at IS NOT NULL AND heartbeat_at < NOW() - 2min`, resets lock fields, sets status back to `pending` or `retrying`. Heartbeat goroutine updates `heartbeat_at = NOW()` every 30s for all jobs `locked_by = workerID`.

## API key format

Keys: `bb_live_<32 hex>` or `bb_test_<32 hex>`. Only SHA-256 hash stored. Raw key returned once at creation. Test mode keys skip real provider calls. Auth: `Authorization: Bearer bb_live_xxxx`.

## Provider interface

```go
type DeliveryProvider interface {
    Channel() domain.Channel
    Name() string
    Send(ctx context.Context, job domain.DeliveryJob) (*ProviderResult, error)
}
```

### SES payload

```json
{ "to": "user@example.com", "subject": "...", "html": "...", "from": "..." }
```

### FCM payload

```json
{ "device_token": "...", "title": "...", "body": "...", "data": {} }
```

## API surface

```
POST   /v1/notifications           create (idempotent)
POST   /v1/notifications/bulk      batch create
GET    /v1/notifications/:id       fetch with delivery statuses
PATCH  /v1/notifications/:id       cancel or reschedule
POST   /v1/notifications/edit      edit content if not yet sent
GET    /v1/notifications           list (status, channel, tag, from, to)
GET    /v1/delivery-jobs/:id       single job status + attempts
GET    /v1/delivery-jobs           list with filters
POST   /v1/api-keys                create (returns raw key once)
GET    /v1/api-keys                list
DELETE /v1/api-keys/:id            revoke
GET    /v1/health                  liveness
```

## Config (environment variables)

```
DATABASE_URL, API_PORT=8080
AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, SES_FROM_EMAIL
FCM_CREDENTIALS_JSON=./credentials/fcm.json
WORKER_ID                        # auto-generated UUID if empty
WORKER_POLL_INTERVAL=5s, WORKER_CONCURRENCY=10
WORKER_HEARTBEAT_INTERVAL=30s, WORKER_STALE_THRESHOLD=2m
WORKER_MAX_RETRIES=3, WORKER_BASE_DELAY=30s, WORKER_MAX_DELAY=1h
```

## Code conventions

- Errors wrapped with context: `fmt.Errorf("notificationRepo.Create: %w", err)`
- No global state — everything injected via constructor. No `init()`.
- Interfaces defined in consuming package, not in a central package
- DTOs in `internal/api/dto/` — never pass domain types to HTTP layer directly
- Context always first argument
- Use `pgx/v5` named parameters (`@param_name` syntax)
- All times UTC; use `time.UTC` explicitly when constructing
- Worker instance ID: UUID generated at startup, passed as `locked_by`

## Build order (greenfield)

1. `db/migrations/` — full schema (api_keys, providers, notifications, delivery_jobs, delivery_attempts + indexes)
2. `db/queries/` — SQL for sqlc, then `make sqlc`
3. `internal/domain/` — all types and status constants (no tags)
4. `internal/repository/` — concrete postgres repo impls wrapping sqlc
5. `config/config.go` — env-based config struct
6. `cmd/api/main.go` — wire + serve (start with `/v1/health`)
7. `internal/api/middleware/auth.go` — Bearer key validation
8. `internal/api/handler/notification.go` — POST /v1/notifications
9. `internal/provider/ses.go`, `fcm.go`, `registry.go`
10. `internal/service/notification.go` — Ingest, Cancel, Reschedule, EditContent (interfaces inline)
11. `cmd/worker/main.go` + `internal/worker/` — poller, executor, heartbeat, scheduler
12. Remaining API handlers and `docker-compose.yml`
