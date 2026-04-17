.PHONY: run build test test-race cover lint tidy fmt vet docker-up docker-down docker-logs

run:
	go run ./cmd/server

build:
	go build -trimpath -o bin/server ./cmd/server

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -n 1

fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

vet:
	go vet ./...

tidy:
	go mod tidy

lint: fmt vet

docker-up:
	cd .. && docker compose up --build -d

docker-down:
	cd .. && docker compose down

docker-logs:
	cd .. && docker compose logs -f backend
