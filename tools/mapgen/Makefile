.PHONY: test test-examples lint

test:
	go test ./...

test-examples:
	cd examples && go generate ./... && go test ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed"; \
		exit 1; \
	}
	golangci-lint run
