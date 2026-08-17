.PHONY: run build test lint vet docker-up docker-down tidy

run: ## Run the BFF locally (reads .env if present via your shell/dotenv tool of choice)
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
