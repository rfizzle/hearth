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
BUF=buf
# What `buf generate` writes; proto-drift fails when it drifts from the commit.
PROTO_GEN_DIRS=internal/rpc/gen App/Packages/HearthKit/Sources/HearthKit/Gen
# astimate is run locally until the binary is published; CI overrides ASTIMATE.
ASTIMATE ?= astimate
ASTIMATE_BASE ?= master
SKILLS_SOURCE ?= ../skills
# `make fmt` rewrites files, so it finds them on disk; dotted directories are
# skipped because they can hold somebody else's working tree.
# Generated code is left as buf writes it (goimports would regroup it), and
# proto-check holds it to the generator instead.
PROJECT_GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./.*" -not -path "./internal/rpc/gen/*")
# The gate only reads, and what it reads is what a commit would carry.
# Tracked plus new untracked files, so a file passes the gate before it is
# committed as well as after.
TRACKED_GOFILES=$(shell git ls-files --cached --others --exclude-standard '*.go' ':!:internal/rpc/gen/**')
PROJECT_PACKAGES=$(shell go list ./...)
SWIFT=swift
SWIFTLINT=swiftlint
APP_DIR=App
SWIFT_TEST_PACKAGES=$(APP_DIR)/Packages/HearthKit $(APP_DIR)/Packages/HearthDesign $(APP_DIR)/Packages/HearthUI $(APP_DIR)/Packages/HearthFeatures $(APP_DIR)/Packages/HearthNative
# Generated Swift is left as buf writes it, as generated Go is.
SWIFT_SOURCES=$(shell find $(APP_DIR) -name '*.swift' -not -path '*/.build/*' -not -path '*/Gen/*')
TOKENS_FILE=$(APP_DIR)/Packages/HearthDesign/Sources/HearthDesign/Resources/tokens.json
DIST_DIR=$(CURDIR)/dist

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-s -w -X main.version=$(VERSION)

.PHONY: all build proto proto-check proto-drift fmt fmt-check vet lint test test-contract test-integration docs-check vulncheck astimate ci prepush hooks help swift-fmt swift-format-check swift-lint swift-test tokens-check gallery-build gallery-open gallery-shot gallery-record ci-swift app-build app-sign notarize app-verify cache-dir

all: help

cache-dir:
	@mkdir -p $(CACHE_DIR)/tmp

## Build:
build: cache-dir ## Build the hearthd binary for this platform
	@$(HERMETIC_ENV) CGO_ENABLED=0 $(GOCMD) build -ldflags "$(LDFLAGS)" -o $(APP_NAME) ./cmd/hearthd

proto: cache-dir ## Regenerate the Go and Swift code from proto/ (needs buf and the network)
	@$(HERMETIC_ENV) $(BUF) generate

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

skills-sync: ## Refresh the shared skills from $(SKILLS_SOURCE); add new ones with SKILLS="a b" (local only)
	@$(SKILLS_SOURCE)/scripts/sync -C . $(SKILLS)

skills-check: cache-dir ## Fail if a vendored shared skill differs from the copy recorded in .agents/skills/shared.sum
	@$(HERMETIC_ENV) $(GOCMD) run ./scripts/skillscheck

# Offline: lint, and breaking changes against master, which is skipped,
# saying so, only while master carries no proto.
proto-check: cache-dir ## Lint the contract and check it against master for breaking changes (offline)
	@command -v $(BUF) >/dev/null 2>&1 || { echo "buf not found (install: go install github.com/bufbuild/buf/cmd/buf@v1.73.0)"; exit 1; }
	@$(HERMETIC_ENV) $(BUF) lint
	@# buf drops the empty fsmonitor value when it runs git; false means the same.
	@if git cat-file -e master:buf.yaml 2>/dev/null; then \
		$(HERMETIC_ENV) GIT_CONFIG_VALUE_0=false $(BUF) breaking --against '.git#branch=master'; \
	else \
		echo "no proto on master yet, breaking check skipped"; \
	fi

# The generators are remote buf plugins, so this needs the network, and the
# BSR rate-limits anonymous generation; it is kept out of ci for that reason.
proto-drift: cache-dir ## Regenerate with buf's remote plugins and fail on drift (needs the network; set BUF_TOKEN to lift the BSR rate limit)
	@command -v $(BUF) >/dev/null 2>&1 || { echo "buf not found (install: go install github.com/bufbuild/buf/cmd/buf@v1.73.0)"; exit 1; }
	@$(HERMETIC_ENV) $(BUF) generate
	@git diff --exit-code -- $(PROTO_GEN_DIRS) || { echo "Generated code drifted from proto/. Run make proto and commit."; exit 1; }
	@untracked="$$(git ls-files --others --exclude-standard -- $(PROTO_GEN_DIRS))"; \
	if [ -n "$$untracked" ]; then echo "Generated files not tracked. Run make proto and add them:"; echo "$$untracked"; exit 1; fi

## Test:
# Cacheable on purpose: nothing here adds -count=1.
test: cache-dir ## Run the hermetic tier: no listener, daemon or network
	@$(HERMETIC_ENV) $(GOTEST) -mod=readonly $(PROJECT_PACKAGES)

test-contract: cache-dir ## Run the tests behind the contract tag
	@$(HERMETIC_ENV) $(GOTEST) -mod=readonly -tags=contract -count=1 -v $(PROJECT_PACKAGES)
	@if command -v $(SWIFT) >/dev/null 2>&1 && [ "$$(uname)" = Darwin ]; then \
		HEARTH_CONTRACT=1 $(SWIFT) test --package-path $(APP_DIR)/Packages/HearthNative --filter KeychainContractTests; \
	fi

test-integration: cache-dir ## Run the tests behind the integration tag (may need the network or credentials)
	@$(HERMETIC_ENV) $(GOTEST) -mod=readonly -tags=integration -count=1 -v $(PROJECT_PACKAGES)

## Swift:
# Each Swift target refuses to run without its tool, as fmt-check does: a gate
# that skips itself reports the same green as one that passed.
swift-fmt: ## Rewrite every Swift file with Apple's swift format
	@command -v $(SWIFT) >/dev/null 2>&1 || { echo "swift not found (install Xcode)"; exit 1; }
	@$(SWIFT) format format -i --configuration $(APP_DIR)/.swift-format $(SWIFT_SOURCES)

swift-format-check: ## Fail if any Swift file is not as Apple's swift format leaves it
	@command -v $(SWIFT) >/dev/null 2>&1 || { echo "swift not found (install Xcode)"; exit 1; }
	@$(SWIFT) format lint --strict --configuration $(APP_DIR)/.swift-format $(SWIFT_SOURCES)

swift-lint: ## Run SwiftLint: the size rules and the raw style literal rule
	@command -v $(SWIFTLINT) >/dev/null 2>&1 || { echo "swiftlint not found (install: brew install swiftlint)"; exit 1; }
	@cd $(APP_DIR) && $(SWIFTLINT) lint --strict --quiet --config .swiftlint.yml

# The root package is built and runs the app target's tests; every local
# package runs its tests in its own .build.
swift-test: ## Build the app targets and run every Swift package's tests
	@command -v $(SWIFT) >/dev/null 2>&1 || { echo "swift not found (install Xcode)"; exit 1; }
	@$(SWIFT) build --package-path $(APP_DIR)
	@$(SWIFT) test --package-path $(APP_DIR)
	@for p in $(SWIFT_TEST_PACKAGES); do \
		echo "swift test $$p"; \
		$(SWIFT) test --package-path $$p || exit 1; \
	done

tokens-check: cache-dir ## Fail if a token pairing is below WCAG 2.2 AA contrast in light or dark
	@$(HERMETIC_ENV) $(GOCMD) run ./scripts/tokenscheck $(TOKENS_FILE)

gallery-build: ## Build the gallery app: every component in every state
	@command -v $(SWIFT) >/dev/null 2>&1 || { echo "swift not found (install Xcode)"; exit 1; }
	@$(SWIFT) build --package-path $(APP_DIR) --product HearthGallery

# GALLERY_ARGS reach the gallery as launch arguments: --page <Name>, --text
# largest, --window-width <points>. The window is a fixed size by default, and
# macOS is told not to restore the last one, so every capture frames the same.
# A running gallery is quit first: `open` on a running app would only raise it.
GALLERY_ARGS ?= $(if $(PAGE),--page $(PAGE)) -ApplePersistenceIgnoreState YES
GALLERY_SHOT_DIR=$(CACHE_DIR)/gallery

gallery-open: gallery-build ## Build, bundle and open the gallery (GALLERY_ARGS="--page Avatar")
	@mkdir -p $(DIST_DIR)
	@scripts/app/gallery.sh "$$($(SWIFT) build --package-path $(APP_DIR) --show-bin-path)" $(DIST_DIR)
	@pkill -x HearthGallery || true
	@sleep 1
	@open $(DIST_DIR)/HearthGallery.app --args $(GALLERY_ARGS)

gallery-shot: ## Capture a gallery page to .cache/gallery/<PAGE>.png (PAGE=MessageRow, needs Screen Recording)
	@test -n "$(PAGE)" || { echo "PAGE is required, e.g. make gallery-shot PAGE=MessageRow"; exit 1; }
	@$(MAKE) --no-print-directory gallery-open
	@sleep 3
	@mkdir -p $(GALLERY_SHOT_DIR)
	@osascript -e 'tell application "HearthGallery" to activate'
	@sleep 1
	@$(SWIFT) scripts/app/windowid.swift "Hearth Gallery" > $(GALLERY_SHOT_DIR)/.windowid
	@screencapture -x -o -l "$$(cat $(GALLERY_SHOT_DIR)/.windowid)" $(GALLERY_SHOT_DIR)/$(PAGE).png
	@sips -Z 1600 $(GALLERY_SHOT_DIR)/$(PAGE).png >/dev/null
	@echo "captured $(GALLERY_SHOT_DIR)/$(PAGE).png"

# The Motion page plays each change on launch with --play; the recording is the
# window's region, since screencapture -l is stills only. Seconds cover the
# whole sequence.
RECORD_SECONDS ?= 45

gallery-record: ## Record the Motion page playing to .cache/gallery/Motion.mov (needs Screen Recording)
	@$(MAKE) --no-print-directory gallery-open GALLERY_ARGS="--page Motion --play -ApplePersistenceIgnoreState YES"
	@mkdir -p $(GALLERY_SHOT_DIR)
	@until $(SWIFT) scripts/app/windowid.swift --bounds "Hearth Gallery" >/dev/null 2>&1; do sleep 0.3; done
	@osascript -e 'tell application "HearthGallery" to activate'
	@screencapture -v -V $(RECORD_SECONDS) -x -R "$$($(SWIFT) scripts/app/windowid.swift --bounds "Hearth Gallery")" $(GALLERY_SHOT_DIR)/Motion.mov
	@echo "recorded $(GALLERY_SHOT_DIR)/Motion.mov"

ci-swift: swift-format-check swift-lint swift-test gallery-build ## Run every Swift check CI runs (macOS only)

## App:
# The helper is built for the app's platform whatever the host, and the bundle
# is assembled by a script because there is no Xcode project.
app-build: cache-dir ## Build dist/Hearth.app with the helper inside (VERSION sets the bundle version)
	@command -v $(SWIFT) >/dev/null 2>&1 || { echo "swift not found (install Xcode)"; exit 1; }
	@mkdir -p $(DIST_DIR)
	@$(HERMETIC_ENV) GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GOCMD) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/hearthd ./cmd/hearthd
	@$(SWIFT) build -c release --package-path $(APP_DIR) --product Hearth
	@scripts/app/bundle.sh $(APP_DIR)/.build/release $(DIST_DIR)/hearthd $(VERSION) $(DIST_DIR)

## Release:
app-sign: ## Sign helper then app (MACOS_SIGN_P12, MACOS_SIGN_PASSWORD; ad-hoc when unset)
	@CACHE_DIR=$(CACHE_DIR) scripts/app/sign.sh $(DIST_DIR)/Hearth.app

notarize: ## Notarise and staple (MACOS_NOTARY_ISSUER_ID, MACOS_NOTARY_KEY_ID, MACOS_NOTARY_KEY)
	@CACHE_DIR=$(CACHE_DIR) scripts/app/notarize.sh $(DIST_DIR)/Hearth.app

app-verify: ## Check both signatures carry the hardened runtime, LC_BUILD_VERSION, and the staple
	@scripts/app/verify.sh $(DIST_DIR)/Hearth.app

## CI:
# Order matters: the cheap checks fail first.
ci: fmt-check vet lint test docs-check skills-check proto-check tokens-check astimate ## Run every check CI runs

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
