# DocLens Makefile
# Convention: targets are verbs. Each target prints a short banner and
# delegates to the underlying tool. Compose detection handles both the v2
# plugin (`docker compose`) and legacy `docker-compose`.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# Detect Docker Compose (v2 plugin preferred, legacy fallback).
COMPOSE := $(shell \
	if docker compose version >/dev/null 2>&1; then echo "docker compose"; \
	elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; \
	else echo ""; \
	fi)

COMPOSE_FILE := infra/docker/docker-compose.dev.yml

.PHONY: help bootstrap dev down logs ps test test-go test-web lint lint-go lint-web format gen migrate clean

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

bootstrap: ## Install all dependencies (JS + Go).
	@echo "==> Installing JS dependencies"
	pnpm install
	@echo "==> Syncing Go workspace"
	go work sync

dev: ## Start the local stack (Postgres, Redis, MinIO, api, worker, web).
	@if [ -z "$(COMPOSE)" ]; then \
		echo "Docker Compose not found. Install Docker Desktop or 'docker compose' plugin."; exit 1; \
	fi
	@echo "==> Starting dev stack via: $(COMPOSE)"
	$(COMPOSE) -f $(COMPOSE_FILE) up --build

down: ## Stop the local stack.
	@$(COMPOSE) -f $(COMPOSE_FILE) down

logs: ## Tail logs from the local stack.
	@$(COMPOSE) -f $(COMPOSE_FILE) logs -f

ps: ## Show running containers.
	@$(COMPOSE) -f $(COMPOSE_FILE) ps

test: test-go test-web ## Run all tests.

test-go: ## Run Go tests across all modules.
	@echo "==> go test"
	@for mod in $$(find services apps -name go.mod -not -path '*/node_modules/*' 2>/dev/null); do \
		dir=$$(dirname $$mod); \
		echo "    -> $$dir"; \
		(cd $$dir && go test ./...); \
	done

test-web: ## Run web tests.
	@if pnpm -r ls --depth -1 2>/dev/null | grep -q "@doclens/"; then \
		pnpm turbo run test; \
	else \
		echo "==> no web packages yet; skipping"; \
	fi

lint: lint-go lint-web ## Run all linters.

lint-go: ## Run golangci-lint across all Go modules.
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not installed. See https://golangci-lint.run/"; exit 1; \
	fi
	@for mod in $$(find services apps -name go.mod 2>/dev/null); do \
		dir=$$(dirname $$mod); \
		echo "==> golangci-lint $$dir"; \
		(cd $$dir && golangci-lint run ./...); \
	done

lint-web: ## Lint TS/JS packages.
	pnpm turbo run lint

format: ## Format JS/TS/JSON/MD with Prettier.
	pnpm format

gen: ## Regenerate code (Go server stubs + TS client) from openapi.yaml.
	@echo "==> oapi-codegen (Go server stubs)"
	@(cd apps/api && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config oapi-codegen.yaml openapi.yaml)
	@echo "==> openapi-typescript (TS client)"
	@pnpm --filter @doclens/api-client gen

migrate: ## Apply database migrations. No-op until M3.
	@echo "Migrations land in Milestone 3."

clean: ## Remove build artifacts and caches.
	rm -rf node_modules .turbo
	@for dir in apps/*/node_modules packages/*/node_modules apps/*/.next; do \
		rm -rf $$dir 2>/dev/null || true; \
	done
	@find . -name '*.tsbuildinfo' -delete 2>/dev/null || true
