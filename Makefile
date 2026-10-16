# vlx developer tasks. Run `make help` for a list.

GO      ?= go
BIN     := bin/vlx
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/rossedman/vlx/internal/version.Version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[35m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build ./bin/vlx
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/vlx

.PHONY: install
install: ## Install vlx into $GOBIN
	CGO_ENABLED=0 $(GO) install -trimpath -ldflags '$(LDFLAGS)' ./cmd/vlx

.PHONY: run-demo
run-demo: build ## Build and explore the demo service
	./$(BIN) demo

.PHONY: test
test: ## Run all tests
	$(GO) test ./...

.PHONY: race
race: ## Run all tests with the race detector
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover: ## Write a coverage report to coverage.html
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

.PHONY: lint
lint: ## Run gofmt, go vet and golangci-lint
	@test -z "$$(gofmt -l . | grep -v '^site/')" || (gofmt -l .; exit 1)
	$(GO) vet ./...
	golangci-lint run

.PHONY: docs-reference
docs-reference: ## Regenerate the command reference in docs/reference/cli
	rm -rf docs/reference/cli
	XDG_CONFIG_HOME=/nonexistent $(GO) run ./cmd/vlx gen-docs docs/reference/cli

.PHONY: docs-deps
docs-deps: ## Install MkDocs and plugins
	pip install -r docs/requirements.txt

.PHONY: docs-serve
docs-serve: docs-reference ## Serve the docs at http://127.0.0.1:8000
	mkdocs serve

.PHONY: docs
docs: docs-reference ## Build the docs site into site/
	mkdocs build --strict

.PHONY: screenshots
screenshots: ## Regenerate TUI screenshots (needs node + Playwright)
	./hack/screenshots.sh

.PHONY: assets
assets: ## Re-render the banner and diagrams (needs node + Playwright)
	./hack/render-assets.sh

.PHONY: release-check
release-check: ## Validate the GoReleaser config
	goreleaser check

.PHONY: snapshot
snapshot: ## Build a local GoReleaser snapshot into dist/
	goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build outputs
	rm -rf bin dist site completions manpages coverage.out coverage.html
