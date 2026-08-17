.PHONY: run build test lint vet docker-up docker-down tidy

# Load .env into every target's environment, if present. `include` parses it
# as Makefile syntax (fine for plain KEY=value lines, which is all .env
# uses); `export` forwards those variables to child processes like `go run`.
ifneq (,$(wildcard .env))
include .env
export
endif

run: ## Run the BFF locally (.env is loaded automatically if present)
	go run ./cmd/server

build: ## Build the server binary
	go build -o bin/server ./cmd/server

test: ## Run tests with race detector
	go test ./... -race

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run/usage/install/)
	golangci-lint run ./...

docker-up: ## Start local Redis (and future local proxy) for dev
	docker compose up -d

docker-down: ## Stop local dev containers
	docker compose down

tidy: ## Sync go.mod/go.sum
	go mod tidy
