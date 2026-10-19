APP_NAME=hearthd
GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
CACHE_DIR=$(CURDIR)/.cache
# Every check runs in one environment: no terminal palette inherited from the
# shell, no executable Git fsmonitor hook, and every tool cache under .cache/.
# The caches stay out of /tmp because a small tmpfs fills up on a cold build.
HERMETIC_ENV=env -u NO_COLOR GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.fsmonitor GIT_CONFIG_VALUE_0= GOCACHE=$(CACHE_DIR)/go-build GOTMPDIR=$(CACHE_DIR)/tmp XDG_CACHE_HOME=$(CACHE_DIR) GOLANGCI_LINT_CACHE=$(CACHE_DIR)/golangci-lint
# The build tags the other test tiers live behind. vet reads them so a file
# that stopped compiling fails in the gate rather than in the one job that
# selects it.
TIER_TAGS=contract,integration
# gofmt ships with the toolchain but is not always on PATH; fall back to GOROOT.
GOFMT ?= $(shell command -v gofmt 2>/dev/null || echo "$$(go env GOROOT)/bin/gofmt")
GOIMPORTS=goimports
GOLANGCI_LINT=golangci-lint
GOVULNCHECK=govulncheck
# astimate is run locally until the binary is published; CI overrides ASTIMATE.
ASTIMATE ?= astimate
ASTIMATE_BASE ?= master
# `make fmt` rewrites files, so it finds them on disk; dotted directories are
# skipped because they can hold somebody else's working tree.
PROJECT_GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.*")
# The gate only reads, and what it reads is what a commit would carry.
TRACKED_GOFILES=$(shell git ls-files '*.go')
PROJECT_PACKAGES=$(shell go list ./...)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-s -w -X main.version=$(VERSION)

.PHONY: all build fmt fmt-check vet lint test test-contract test-integration docs-check vulncheck astimate ci prepush hooks help swift-format-check swift-lint swift-test tokens-check gallery-build cache-dir

all: help

cache-dir:
	@mkdir -p $(CACHE_DIR)/tmp

## Build:
build: cache-dir ## Build the hearthd binary for this platform
	@$(HERMETIC_ENV) CGO_ENABLED=0 $(GOCMD) build -ldflags "$(LDFLAGS)" -o $(APP_NAME) ./cmd/hearthd

## Check:
fmt: ## Rewrite every Go file with gofmt and goimports
	@$(GOFMT) -e -s -w $(PROJECT_GOFILES)
	@if command -v $(GOIMPORTS) >/dev/null 2>&1; then \
		$(GOIMPORTS) -e -format-only -w $(PROJECT_GOFILES); \
	else \
		echo "goimports not found, skipping (install: go install golang.org/x/tools/cmd/goimports@latest)"; \
	fi

# Both checks are hard: this refuses to run when a formatter is missing,
# because a gate that skips itself reports the same green as one that passed.
fmt-check: ## Fail if any tracked Go file is not gofmt- and goimports-clean
	@command -v $(GOFMT) >/dev/null 2>&1 || { echo "gofmt not found at $(GOFMT)"; exit 1; }
	@drift="$$($(GOFMT) -e -s -l $(TRACKED_GOFILES))"; \
	if [ -n "$$drift" ]; then echo "Not gofmt-clean. Run make fmt:"; echo "$$drift"; exit 1; fi
	@command -v $(GOIMPORTS) >/dev/null 2>&1 || { echo "goimports not found (install: go install golang.org/x/tools/cmd/goimports@latest)"; exit 1; }
	@drift="$$($(GOIMPORTS) -e -format-only -l $(TRACKED_GOFILES))"; \
	if [ -n "$$drift" ]; then echo "Imports are not grouped as goimports leaves them. Run make fmt:"; echo "$$drift"; exit 1; fi

vet: cache-dir ## Run go vet over every tier's files
	@$(HERMETIC_ENV) $(GOVET) -mod=readonly -tags $(TIER_TAGS) $(PROJECT_PACKAGES)

lint: cache-dir ## Run golangci-lint
	@$(HERMETIC_ENV) $(GOLANGCI_LINT) run --modules-download-mode=readonly ./...

vulncheck: cache-dir ## Fail on a known vulnerability the code reaches (needs govulncheck and the network)
	@$(HERMETIC_ENV) $(GOVULNCHECK) ./...

astimate: ## Run the astimate quality gate against the base branch
	@$(ASTIMATE) check --base $(ASTIMATE_BASE)

docs-check: cache-dir ## Verify every docs/ citation resolves and no story or epic id leaks into docs or code
	@$(HERMETIC_ENV) $(GOCMD) run ./scripts/docscheck

## Test:
# Cacheable on purpose: nothing here adds -count=1.
test: cache-dir ## Run the hermetic tier: no listener, daemon or network
	@$(HERMETIC_ENV) $(GOTEST) -mod=readonly $(PROJECT_PACKAGES)

test-contract: cache-dir ## Run the tests behind the contract tag
	@$(HERMETIC_ENV) $(GOTEST) -mod=readonly -tags=contract -count=1 -v $(PROJECT_PACKAGES)

test-integration: cache-dir ## Run the tests behind the integration tag (may need the network or credentials)
	@$(HERMETIC_ENV) $(GOTEST) -mod=readonly -tags=integration -count=1 -v $(PROJECT_PACKAGES)

## Swift:
swift-format-check: ## Placeholder, not yet: lands with S-0002
	@echo "not yet: lands with S-0002"

swift-lint: ## Placeholder, not yet: lands with S-0002
	@echo "not yet: lands with S-0002"

swift-test: ## Placeholder, not yet: lands with S-0002
	@echo "not yet: lands with S-0002"

tokens-check: ## Placeholder, not yet: lands with S-0002
	@echo "not yet: lands with S-0002"

gallery-build: ## Placeholder, not yet: lands with S-0002
	@echo "not yet: lands with S-0002"

## CI:
# Order matters: the cheap checks fail first.
ci: fmt-check vet lint test docs-check astimate ## Run every check CI runs

prepush: ## Run the ci targets before a push
	@$(MAKE) --no-print-directory ci

# Opt-in: nothing installs this on its own. core.hooksPath is shared by every
# worktree of the clone; remove it with `git config --unset core.hooksPath`.
hooks: ## Install the pre-push hook that runs make prepush (opt-in)
	@cur="$$(git config --get core.hooksPath || true)"; \
	if [ -n "$$cur" ] && [ "$$cur" != scripts/git ]; then \
		echo "core.hooksPath is already $$cur; not replacing it"; exit 1; \
	fi; \
	git config core.hooksPath scripts/git; \
	echo "pre-push hook installed (git push --no-verify skips it)"

## Help:
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?##/ {printf "  %-20s%s\n", $$1, $$2}' $(MAKEFILE_LIST)
