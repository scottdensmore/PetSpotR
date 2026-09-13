.DEFAULT_GOAL := help

SHELL := /usr/bin/env bash
GOTOOLCHAIN ?= go1.26.5
TOFU ?= $(shell if command -v tofu >/dev/null 2>&1; then echo "tofu"; elif command -v mise >/dev/null 2>&1; then echo "mise exec -- tofu"; else echo "tofu"; fi)
YAMLLINT ?= $(shell if command -v yamllint >/dev/null 2>&1; then echo "yamllint"; elif command -v mise >/dev/null 2>&1; then echo "mise exec -- yamllint"; else echo "yamllint"; fi)

.PHONY: help
help: ## Show this help message
	@echo "PetSpotR Developer Automation"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: vet
vet: ## Run Go static analysis (go vet)
	@echo "==> Running go vet..."
	@GOTOOLCHAIN=$(GOTOOLCHAIN) go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	@echo "==> Running golangci-lint..."
	@golangci-lint run

.PHONY: test
test: ## Run unit tests with race detector and coverage
	@echo "==> Running go test (-race -cover)..."
	@GOTOOLCHAIN=$(GOTOOLCHAIN) go test -race -cover ./...

.PHONY: infra-check
infra-check: ## Check OpenTofu formatting and validate configuration
	@echo "==> Checking OpenTofu configuration..."
	@cd infra/opentofu && $(TOFU) fmt -check -recursive && $(TOFU) init -backend=false >/dev/null 2>&1 && $(TOFU) validate

.PHONY: yamllint
yamllint: ## Validate Cloud Run manifests
	@echo "==> Running yamllint on manifests..."
	@$(YAMLLINT) -s deploy/cloudrun/

.PHONY: verify
verify: vet lint infra-check yamllint test ## Run complete local verification suite
	@echo "==> All local checks passed successfully!"

.PHONY: emulators-up
emulators-up: ## Start Firestore, PubSub, and Auth emulators and initialize topics
	@echo "==> Starting local emulators..."
	@docker compose up -d firestore-emulator pubsub-emulator auth-emulator
	@./scripts/init-emulators.sh

.PHONY: emulators-down
emulators-down: ## Stop local emulators and clean up volumes
	@echo "==> Stopping local emulators..."
	@docker compose down --volumes --remove-orphans

.PHONY: cascade
cascade: ## Verify full report-to-match event cascade against emulators
	@echo "==> Verifying emulator cascade..."
	@./scripts/verify-cascade.sh

.PHONY: playwright
playwright: ## Run Playwright API journeys against local services
	@echo "==> Running Playwright API journeys..."
	@cd tests/playwright && npm ci && npx playwright test

.PHONY: setup-hooks
setup-hooks: ## Install local git pre-push verification hook
	@./scripts/setup-git-hooks.sh
