# GAIA — Build & Test automation
# Run `make help` for available targets.

.PHONY: help build test test-race lint clean

BINARY := gaia.exe
CMD    := ./cmd/gaia

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the GAIA binary
	go build -o $(BINARY) $(CMD)

test: ## Run all unit tests
	go test ./... -count=1

test-race: ## Run all tests with the race detector
	go test -race ./... -count=1

lint: ## Run golangci-lint (requires global install)
	golangci-lint run

clean: ## Remove build artifacts
	rm -f $(BINARY)
	go clean -cache -testcache
