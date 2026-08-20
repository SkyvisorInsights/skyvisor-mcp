.PHONY: hooks fmt lint test build

hooks: ## Install versioned git hooks (.githooks → core.hooksPath)
	bash scripts/install-hooks.sh

fmt: ## Format Go sources with gofumpt
	go tool gofumpt -w .

lint: ## Run golangci-lint (same command the pre-commit hook and CI run)
	golangci-lint run

test: ## Run the test suite
	go test ./... -count=1

build: ## Build all packages
	go build ./...
