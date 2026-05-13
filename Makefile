-include .env
export

GO ?= go
SQLC ?= sqlc
MIGRATE ?= migrate

DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/beaconbird?sslmode=disable
MIGRATIONS_DIR := db/migrations

.PHONY: run-api run-worker migrate-up migrate-down migrate-create sqlc test lint build docker-up

run-api:
	$(GO) run ./cmd/api

run-worker:
	$(GO) run ./cmd/worker

migrate-up:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=create_table" && exit 1)
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

sqlc:
	$(SQLC) generate

test:
	env GOCACHE=/tmp/gocache $(GO) test ./...

lint:
	golangci-lint run

build:
	mkdir -p bin
	env GOCACHE=/tmp/gocache $(GO) build -o bin/api ./cmd/api
	env GOCACHE=/tmp/gocache $(GO) build -o bin/worker ./cmd/worker

docker-up:
	docker compose up -d
