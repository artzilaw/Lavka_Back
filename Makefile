.PHONY: run test test-coverage migrate-up migrate-down clean

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

DATABASE_URL ?= postgres://lavka:lavka_pass@localhost:5432/lavka?sslmode=disable

run:
	go run ./cmd/server/main.go --config=./config/local.json

test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out

migrate-up:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000001_init_schema.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000002_seed_initial_data.up.sql

migrate-down:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000002_seed_initial_data.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000001_init_schema.down.sql

clean:
	rm -f coverage.out
	rm -rf bin/ storage/
