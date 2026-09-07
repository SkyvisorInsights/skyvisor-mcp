.PHONY: hooks fmt lint test build verify-pinned

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

verify-pinned: ## Build and test as CI does: no go.work, so go.mod pins decide
	# go.work resolves sibling modules to the local checkout, hiding a stale
	# skyvisor-go-shared pin that the workspace-free Docker build would hit.
	GOWORK=off go build ./...
	GOWORK=off go test ./... -count=1
