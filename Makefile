.PHONY: run build test test-integration coverage lint fmt-check vet docker-up docker-down tidy

# cmd/server is composition-root wiring (reads env, constructs real
# dependencies, calls ListenAndServe) — there's no meaningful behavior in it
# to unit test independent of an actual process boot, so it's excluded from
# the coverage gate. Everything with real logic lives in internal/... and is
# included.
COVERAGE_THRESHOLD := 75

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

test-integration: ## Run tests gated behind the `integration` build tag (need a real Redis at REDIS_URL, default redis://localhost:6379)
	go test -tags=integration ./... -v

coverage: ## Run tests with coverage and enforce COVERAGE_THRESHOLD (internal/... only; see note above)
	go test ./internal/... -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	@pct=$$(go tool cover -func=coverage.out | tail -1 | awk '{print substr($$3, 1, length($$3)-1)}'); \
	awk -v pct="$$pct" -v threshold="$(COVERAGE_THRESHOLD)" 'BEGIN { \
		if (pct+0 < threshold+0) { print "FAIL: coverage " pct "% is below threshold " threshold "%"; exit 1 } \
		else { print "OK: coverage " pct "% meets threshold " threshold "%" } \
	}'

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run/usage/install/)
	golangci-lint run ./...

fmt-check: ## Fail if any file isn't gofmt'd (check-only, never writes — CI uses this instead of `gofmt -w`)
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi

docker-up: ## Start local Redis (and future local proxy) for dev
	docker compose up -d

docker-down: ## Stop local dev containers
	docker compose down

tidy: ## Sync go.mod/go.sum
	go mod tidy
