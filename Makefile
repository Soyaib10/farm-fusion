.PHONY: all build test test-race vet tidy clean docker-up docker-down run-api run-scheduler run-worker

BINARY_DIR := bin

all: vet test build

build:
	@mkdir -p $(BINARY_DIR)
	go build -trimpath -o $(BINARY_DIR)/api ./cmd/api
	go build -trimpath -o $(BINARY_DIR)/scheduler ./cmd/scheduler
	go build -trimpath -o $(BINARY_DIR)/worker ./cmd/worker

test:
	go test ./...

test-race:
	go test -race -cover ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf $(BINARY_DIR)

docker-up:
	docker compose up -d

docker-down:
	docker compose down

run-api:
	go run ./cmd/api

run-scheduler:
	go run ./cmd/scheduler

run-worker:
	go run ./cmd/worker
