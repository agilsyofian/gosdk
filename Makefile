.PHONY: build test lint vet fmt fmt-fix vuln check tidy

GO       ?= go

build: ## Build all packages
	@find . -name "go.mod" -not -path "*/vendor/*" -execdir $(GO) build ./... \;

fmt: ## Check formatting (fails if gofmt would change anything)
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:"; gofmt -l .; exit 1)

fmt-fix: ## Automatically format all files
	@gofmt -w .

vet: ## Run go vet across all modules
	@find . -name "go.mod" -not -path "*/vendor/*" -execdir $(GO) vet ./... \;

lint: fmt vet ## Run gofmt + go vet + golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || \
		echo "golangci-lint not installed locally — install it before pushing, see .instructions/coding-style.md"

test: ## Run unit tests across all modules
	@find . -name "go.mod" -not -path "*/vendor/*" -execdir $(GO) test -race -cover ./... \;

vuln: ## Run govulncheck (see .instructions/security.md)
	@command -v govulncheck >/dev/null 2>&1 && govulncheck ./... || \
		echo "govulncheck not installed locally"

tidy: ## Tidy all module dependencies
	@find . -name "go.mod" -not -path "*/vendor/*" -execdir $(GO) mod tidy \;

check: lint test vuln ## Run everything required before considering a task complete
