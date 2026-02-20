#! make
# Terraform Provider Arcane

# ──────────────────────────────────────────────────────────────
# Configuration
# ──────────────────────────────────────────────────────────────

SHELL := /bin/bash
.DEFAULT_GOAL := help

BINARY_NAME   := terraform-provider-arcane
VERSION       ?= dev
OS_ARCH       := $(shell go env GOOS)_$(shell go env GOARCH)
BUILD_DIR     := $(CURDIR)/dist
REPORTS_DIR   := $(CURDIR)/target/reports
PLUGIN_PATH   := $(HOME)/.terraform.d/plugins/registry.terraform.io/darshan-rambhia/arcane/$(VERSION)/$(OS_ARCH)
ARCANE_URL    ?= http://localhost:8000
GO            := go
LOCAL_PKG     := github.com/darshan-rambhia/terraform-provider-arcane

# Build metadata
GIT_COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE    := $(shell date +%Y-%m-%dT%H:%M:%S)

# Silence command echo unless VERBOSE=1
$(VERBOSE).SILENT:

# Only targets that could shadow real files/directories need .PHONY.
# docs/ and tests/ exist; build/, clean, install, generate are common conventions.
.PHONY: build clean docs generate install test

# ──────────────────────────────────────────────────────────────
# Core
# ──────────────────────────────────────────────────────────────

help: ## Show this list of commands
	@printf "Terraform Provider Arcane — available commands:\n\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(firstword $(MAKEFILE_LIST)) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the provider binary to dist/
	@mkdir -p $(BUILD_DIR)
	$(GO) build -ldflags="-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)
	@printf "Built \033[32m%s\033[0m (%s @ %s)\n" "$(BINARY_NAME)" "$(GIT_COMMIT)" "$(VERSION)"

install: build ## Alias for build

clean: ## Remove build and test artifacts
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf target/
	rm -f .terraformrc
	@printf "Cleanup complete\n"

# ──────────────────────────────────────────────────────────────
# Testing
# ──────────────────────────────────────────────────────────────

test: ## Run tests with gotestsum (testdox + coverage + reports)
	@mkdir -p $(REPORTS_DIR)
	TF_ACC=1 $(GO) run gotest.tools/gotestsum@latest \
		--format testdox \
		--junitfile $(REPORTS_DIR)/test-report.xml \
		--jsonfile $(REPORTS_DIR)/test-report.json \
		-- \
		-v \
		-timeout 30m \
		-coverprofile=$(REPORTS_DIR)/coverage.out \
		./internal/...
	@$(GO) tool cover -func=$(REPORTS_DIR)/coverage.out | tail -1

testacc: test ## Run acceptance tests (alias for test)

test-plain: ## Run tests with plain go test (no gotestsum)
	TF_ACC=1 $(GO) test -v ./internal/... -timeout 30m

test-coverage: test ## Open coverage report in browser
	@$(GO) tool cover -html=$(REPORTS_DIR)/coverage.out

# ──────────────────────────────────────────────────────────────
# Code Quality
# ──────────────────────────────────────────────────────────────

fmt: ## Format code with goimports
	$(GO) run golang.org/x/tools/cmd/goimports@latest -w -local $(LOCAL_PKG) .

lint: ## Run golangci-lint
	$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run

lint-fix: ## Run golangci-lint with auto-fix
	$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fix

check: fmt lint build ## Quality gate: fmt + lint + build

# ──────────────────────────────────────────────────────────────
# Dependencies
# ──────────────────────────────────────────────────────────────

deps: ## Download and tidy Go modules
	$(GO) mod download
	$(GO) mod verify
	$(GO) mod tidy

# ──────────────────────────────────────────────────────────────
# Documentation
# ──────────────────────────────────────────────────────────────

docs: ## Generate provider documentation
	$(GO) generate ./...
	@if command -v tfplugindocs > /dev/null; then \
		tfplugindocs generate; \
	else \
		printf "tfplugindocs not installed. Run: go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest\n"; \
	fi

# ──────────────────────────────────────────────────────────────
# Code Generation
# ──────────────────────────────────────────────────────────────

.PHONY: generate
generate: fetch-spec generate-spec generate-framework generate-crud ## Run full code generation pipeline

fetch-spec: ## Fetch OpenAPI spec from Arcane instance
	@printf "Fetching OpenAPI spec from %s...\n" "$(ARCANE_URL)"
	@mkdir -p spec
	curl -s -o spec/arcane_openapi.json $(ARCANE_URL)/api/openapi.json || \
		(printf "Error: Could not fetch spec from %s. Is Arcane running?\n" "$(ARCANE_URL)" && exit 1)
	@printf "Spec saved to spec/arcane_openapi.json\n"

generate-spec: ## Generate provider spec from OpenAPI
	@printf "Generating provider spec from OpenAPI...\n"
	@if command -v tfplugingen-openapi > /dev/null; then \
		tfplugingen-openapi generate \
			--config generator/generator_config.yml \
			--output spec/provider_spec.json \
			spec/arcane_openapi.json; \
	else \
		printf "tfplugingen-openapi not installed. Run: go install github.com/hashicorp/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi@latest\n"; \
	fi

generate-framework: ## Generate framework code from provider spec
	@printf "Generating framework code...\n"
	@if command -v tfplugingen-framework > /dev/null; then \
		tfplugingen-framework generate all \
			--input spec/provider_spec.json \
			--output internal/provider/generated; \
	else \
		printf "tfplugingen-framework not installed. Run: go install github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework@latest\n"; \
	fi

generate-crud: ## Generate CRUD logic from templates
	@printf "Generating CRUD logic...\n"
	@if [ -f generator/generate.go ]; then \
		$(GO) run ./generator/... \
			--spec spec/provider_spec.json \
			--templates generator/crud_templates \
			--output internal/provider; \
	else \
		printf "CRUD generator not implemented yet. Using manual implementations.\n"; \
	fi

# ──────────────────────────────────────────────────────────────
# Development
# ──────────────────────────────────────────────────────────────

dev-install: install terraformrc ## Build provider + generate .terraformrc for local dev
	@printf "\nProvider built to %s\n" "$(BUILD_DIR)"
	@printf ".terraformrc created with dev_overrides\n\n"
	@printf "To use in other projects:\n"
	@printf "  cp .terraformrc ~/.terraformrc\n\n"
	@printf "Or set TF_CLI_CONFIG_FILE:\n"
	@printf "  export TF_CLI_CONFIG_FILE=%s/.terraformrc\n" "$(CURDIR)"

terraformrc: ## Generate .terraformrc for local development
	@printf 'provider_installation {\n  dev_overrides {\n    "darshan-rambhia/arcane" = "%s"\n  }\n  direct {}\n}\n' "$(BUILD_DIR)" > .terraformrc
	@printf "Created .terraformrc with dev_overrides pointing to %s\n" "$(BUILD_DIR)"
