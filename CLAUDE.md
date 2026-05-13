# CLAUDE.md

This file is the working reference for agents and developers in this repository.

## What BeaconBird Is

BeaconBird is a Go notification orchestration service backed by PostgreSQL.

- `cmd/api` exposes the HTTP API for creating, editing, cancelling, and listing notifications, delivery jobs, and API keys.
- `cmd/worker` runs the background delivery loop: polling eligible jobs, sending them through providers, heartbeating active locks, and recovering stale work.

Current channel support:

- Push via FCM is implemented.
- Email via SES is implemented.
- Providers are registered by the worker at startup based on config.
- The mock provider is compiled in for both `push` and `email`, but is disabled by default and only fills channels that do not already have a real provider configured.

## Quick Start

Local prerequisites:

- Go installed
- PostgreSQL available locally, or Docker
- `sqlc`
- `golang-migrate` CLI as `migrate`
- `golangci-lint` if you want `make lint`
- FCM credentials file if you want the worker to send real push notifications

Typical local flow:

1. Start PostgreSQL.
2. Set `DATABASE_URL`.
3. Run `make migrate-up`.
4. Run `make run-api`.
5. In another shell, run `make run-worker`.

Docker flow:

1. Put FCM credentials at `./credentials/fcm.json`.
2. Run `make docker-up`.
3. Run migrations separately against the containerized database if needed.

## Make Targets

- `make run-api` runs `./cmd/api`
- `make run-worker` runs `./cmd/worker`
- `make migrate-up` applies all migrations
- `make migrate-down` rolls back one migration
- `make migrate-create name=add_feature` creates a numbered migration pair
- `make sqlc` regenerates sqlc code from `db/queries/`
- `make test` runs `go test ./...`
- `make lint` runs `golangci-lint`
- `make build` builds both binaries into `bin/`
- `make docker-up` starts the compose stack

The `Makefile` loads `.env` automatically if present.

## Environment

Required:

- `DATABASE_URL`

Common API settings:

- `API_PORT` default `8080`

Worker and provider settings:

- `ENABLE_FCM_PROVIDER` default `false`
- `ENABLE_SES_PROVIDER` default `false`
- `ENABLE_MOCK_PROVIDER` default `false`
- `FCM_CREDENTIALS_JSON` default `./credentials/fcm.json`
- `WORKER_ID` auto-generated if unset
- `WORKER_POLL_INTERVAL` default `5s`
- `WORKER_CONCURRENCY` default `10`
- `WORKER_HEARTBEAT_INTERVAL` default `30s`
- `WORKER_STALE_THRESHOLD` default `2m`
- `WORKER_MAX_RETRIES` default `3`
- `WORKER_BASE_DELAY` default `30s`
- `WORKER_MAX_DELAY` default `1h`

SES-related env vars:

- `AWS_REGION`
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `SES_FROM_EMAIL`

SES payload contract for `email` jobs:

- required: `to_email`
- required: `subject`
- one of `body_text` or `body_html` is required
- both `body_text` and `body_html` may be provided together

## Project Structure

High-level dependency direction:

```text
cmd -> internal/api or internal/worker -> internal/service -> internal/repository or internal/provider
```

Important directories:

- `cmd/api` wires the HTTP server, middleware, repositories, and notification service
- `cmd/worker` wires config, database, and the worker runtime entrypoint
- `config` loads environment-based runtime configuration
- `db/migrations` contains schema migrations
- `db/queries` contains sqlc query definitions
- `internal/domain` holds core types and state-machine values
- `internal/api/dto` contains HTTP request and response DTOs
- `internal/api/handler` contains route handlers and handler helpers
- `internal/api/middleware` contains bearer API key authentication
- `internal/service` contains business logic where orchestration is needed
- `internal/repository` contains PostgreSQL-backed repositories and transaction handling
- `internal/repository/sqlc` contains generated sqlc code and should not be edited by hand
- `internal/provider` contains delivery-provider implementations and the provider registry
- `internal/worker` contains the delivery runtime components and provider bootstrap

## Runtime Model

Notification flow:

1. API receives a notification request.
2. Service performs idempotency checks and writes the notification plus one delivery job per delivery target.
3. Worker polls jobs that are ready to run.
4. Worker locks and executes jobs through the provider registry.
5. Attempts are recorded.
6. Success marks the job `sent`.
7. Failure transitions the job to `retrying` or `dead_letter` depending on retry policy.

Delivery job lifecycle:

```text
pending -> processing -> sent
                    \-> failed -> retrying -> processing
                                      \-> dead_letter
pending -> cancelled
```

Worker responsibilities:

- `poller` acquires runnable jobs and fans work out with bounded concurrency
- `executor` sends the job through the provider registry and records the result
- `heartbeat` refreshes lock ownership for in-flight work
- `scheduler` computes retry timing and recovers stale locked jobs
- provider bootstrap enables `fcm`, `ses`, and mock fallback providers from config before the runtime starts

## API Surface

Public health route:

- `GET /v1/health`

Authenticated routes:

- `POST /v1/notifications`
- `POST /v1/notifications/bulk`
- `GET /v1/notifications`
- `GET /v1/notifications/{id}`
- `PATCH /v1/notifications/{id}`
- `POST /v1/notifications/{id}/edit`
- `GET /v1/delivery-jobs`
- `GET /v1/delivery-jobs/{id}`
- `POST /v1/api-keys`
- `GET /v1/api-keys`
- `DELETE /v1/api-keys/{id}`

Authentication:

- Bearer token via `Authorization: Bearer <api-key>`
- API keys use `bb_live_<hex>` or `bb_test_<hex>`
- Only the SHA-256 hash is stored
- Test keys are intended to avoid real provider delivery

## Database Notes

- Write migrations in pairs: `.up.sql` and `.down.sql`
- Keep SQL in `db/queries/` and regenerate with `make sqlc`
- Do not hand-edit generated files in `internal/repository/sqlc`

## Working Rules

- Keep domain types free of transport and database tags
- Map sqlc models to domain types at the repository boundary
- Define interfaces in the consuming package, not in a shared central package
- Keep context as the first argument
- Wrap errors with operation context
- Use UTC consistently for stored times

## Current Gaps And Cautions

- If a channel has no provider registered, the worker will record a failed attempt and dead-letter the job.
- The Docker compose stack starts Postgres, API, and Worker, but migrations still need to be applied explicitly.
