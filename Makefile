.PHONY: build test lint vet fmt vuln check tidy

GO       ?= go
PKG      := ./...

build: ## Build all packages
	$(GO) build $(PKG)

fmt: ## Check formatting (fails if gofmt would change anything)
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:"; gofmt -l .; exit 1)

vet: ## Run go vet
	$(GO) vet $(PKG)

lint: fmt vet ## Run gofmt + go vet + golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || \
		echo "golangci-lint not installed locally — install it before pushing, see .github/instructions/coding-style.md"

test: ## Run unit tests (no real network/DB/broker calls; see testing.md)
	$(GO) test -race -cover $(PKG)

vuln: ## Run govulncheck (see security.md section 7)
	@command -v govulncheck >/dev/null 2>&1 && govulncheck $(PKG) || \
		echo "govulncheck not installed locally — install it before pushing, see .github/instructions/security.md"

tidy: ## Tidy go.mod / go.sum
	$(GO) mod tidy

check: lint test vuln ## Run everything required before considering a task complete (see AI_AGENT.md)
