.PHONY: run run-auth run-catalog run-gateway build-all generate test test-coverage migrate-up migrate-down clean

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

DATABASE_URL ?= postgres://lavka:lavka_pass@localhost:5432/lavka?sslmode=disable

run:
	go run ./cmd/server/main.go --config=./config/local.json

run-auth:
	go run ./cmd/auth-service/main.go --config=./config/local.json

run-catalog:
	go run ./cmd/catalog-service/main.go --config=./config/local.json

run-gateway:
	AUTH_GRPC_ADDR=localhost:50051 CATALOG_GRPC_ADDR=localhost:50052 go run ./cmd/server/main.go --config=./config/local.json

build-all:
	mkdir -p bin
	go build -o bin/lavka-server ./cmd/server
	go build -o bin/lavka-auth ./cmd/auth-service
	go build -o bin/lavka-catalog ./cmd/catalog-service

generate:
	go run github.com/mailru/easyjson/easyjson -all internal/domain/models/ad.go
	go run github.com/mailru/easyjson/easyjson -all internal/domain/models/user.go

test:
	go test -v ./...

test-coverage:
	go test -coverpkg=./internal/... -coverprofile=coverage.raw.out ./internal/...
	grep -v -E '(_easyjson\.go|mock_.*\.go)' coverage.raw.out > coverage.out
	rm -f coverage.raw.out
	go tool cover -func=coverage.out

migrate-up:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000001_init_schema.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000002_seed_initial_data.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000003_favorites.up.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000004_rk4_promotion_and_price_history.up.sql

migrate-down:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000004_rk4_promotion_and_price_history.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000003_favorites.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000002_seed_initial_data.down.sql
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f ./db/migrations/000001_init_schema.down.sql

clean:
	rm -f coverage.out coverage.raw.out
	rm -rf bin/ storage/
