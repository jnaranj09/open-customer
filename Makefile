# Developer and CI entry point (design doc item 2). Every Go command goes through make: on a clean
# checkout web/build is missing and web/embed.go does not compile until web-stub has run.

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := build

# Pins (design doc "Pins"). A bump changes every copy in one commit; check-pins keeps them equal.
PG_IMAGE := postgres:18.6-alpine
GOLANGCI_LINT_VERSION := v2.15.0
SQLC_IMAGE := sqlc/sqlc:1.31.1
export PG_IMAGE

VERSION ?= $(or $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//'),dev)

# Never let Go walk into web/node_modules (some npm packages ship .go files).
GO_PKGS := $(shell go list -e ./... | grep -v /node_modules/)
# golangci-lint takes directories, not import paths.
GO_DIRS := $(shell go list -e -f '{{.Dir}}' ./... | grep -v /node_modules/ | sed 's|^$(CURDIR)|.|')

# CI installs golangci-lint with its action and sets GOLANGCI_LINT=golangci-lint.
GOLANGCI_LINT ?= bin/tools/golangci-lint-$(GOLANGCI_LINT_VERSION)/golangci-lint

# Unreachable functions `deadcode -test` may report; matched exactly, so it only falls. 1 at P0:
# clock.Real.Now, which serve starts using in P1 (then lower this to 0).
DEADCODE_BUDGET := 1

SHELL_FILES := $(shell find . -name '*.sh' -not -path './web/node_modules/*' -not -path './.git/*')

PNPM := cd web && pnpm

.PHONY: web-stub web build test vuln lint check-pins image node-check

web-stub:
	@test -f web/build/index.html || { mkdir -p web/build && echo 'Run make web' > web/build/index.html; }

# The local Node must be the pinned one (engines.node); with fnm: `fnm use` in web/.
node-check:
	@want=$$(sed -n 's/.*"node": "\(.*\)".*/\1/p' web/package.json); have=$$(node --version | sed 's/^v//'); \
	if [ "$$want" != "$$have" ]; then echo "node $$have, but web/package.json pins $$want (fnm use $$want)" >&2; exit 1; fi

web: node-check
	$(PNPM) build
# vite.config.ts reads VERSION from the environment.
web: export VERSION := $(VERSION)

build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/open-customer ./cmd/open-customer

test: web-stub node-check
	go test $(GO_PKGS)
	$(PNPM) test

vuln: web-stub
	go tool govulncheck $(GO_PKGS)

$(GOLANGCI_LINT):
	GOBIN=$(CURDIR)/$(dir $@) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: web-stub node-check $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run $(GO_DIRS)
	@n=$$(go tool deadcode -test $(GO_PKGS) | tee /dev/stderr | wc -l); \
	if [ "$$n" -ne $(DEADCODE_BUDGET) ]; then \
		echo "deadcode: $$n unreachable functions, DEADCODE_BUDGET is $(DEADCODE_BUDGET) (remove the new one, or lower the budget)" >&2; exit 1; fi
	shellcheck $(SHELL_FILES)
	$(PNPM) run check
	$(PNPM) run lint

check-pins:
	PG_IMAGE=$(PG_IMAGE) GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) scripts/check-pins.sh

image:
	docker build --build-arg VERSION=$(VERSION) -t open-customer:$(VERSION) .
