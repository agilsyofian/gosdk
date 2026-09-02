.DEFAULT_GOAL := help

GO             ?= go
MIGRATIONS_DIR ?= migrations
NAME           ?=
VERSION        ?=
STEPS          ?= 1
STATE          ?= applied

.PHONY: help migrate-create migrate-up migrate-up-to migrate-down migrate-status migrate-force

help: ## Show available migration commands
	@awk 'BEGIN {FS = ":.*##"}; /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

migrate-create: ## Create a migration pair; use NAME=create_orders_table
	@test -n "$(NAME)" || (echo "NAME is required: make migrate-create NAME=create_orders_table"; exit 1)
	@MIGRATIONS_DIR="$(MIGRATIONS_DIR)" $(GO) run ./cmd/migrate create "$(NAME)"

migrate-up: ## Apply every pending migration
	@MIGRATIONS_DIR="$(MIGRATIONS_DIR)" $(GO) run ./cmd/migrate up

migrate-up-to: ## Apply through VERSION; use VERSION=20260829210800
	@test -n "$(VERSION)" || (echo "VERSION is required: make migrate-up-to VERSION=20260829210800"; exit 1)
	@MIGRATIONS_DIR="$(MIGRATIONS_DIR)" $(GO) run ./cmd/migrate up "$(VERSION)"

migrate-down: ## Roll back STEPS migrations; defaults to 1
	@MIGRATIONS_DIR="$(MIGRATIONS_DIR)" $(GO) run ./cmd/migrate down "$(STEPS)"

migrate-status: ## Display migration state
	@MIGRATIONS_DIR="$(MIGRATIONS_DIR)" $(GO) run ./cmd/migrate status

migrate-force: ## Force VERSION to STATE=applied or STATE=unapplied
	@test -n "$(VERSION)" || (echo "VERSION is required: make migrate-force VERSION=20260829210800 STATE=applied"; exit 1)
	@test "$(STATE)" = "applied" || test "$(STATE)" = "unapplied" || (echo "STATE must be applied or unapplied"; exit 1)
	@MIGRATIONS_DIR="$(MIGRATIONS_DIR)" $(GO) run ./cmd/migrate force "$(VERSION)" "$(STATE)"
