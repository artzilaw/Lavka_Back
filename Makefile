.PHONY: run test test-coverage clean

run:
	go run ./cmd/server/main.go --config=./config/local.json

test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out

clean:
	rm -f coverage.out
	rm -rf bin/ storage/
