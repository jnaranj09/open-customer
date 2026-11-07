# Developer and CI entry point (design doc item 2). Every Go command goes through make: on a clean
# checkout web/build is missing and web/embed.go does not compile until web-stub has run.

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := build

# Pins (design doc "Pins"). A bump changes every copy in one commit; check-pins keeps them equal.
PG_IMAGE := postgres:18.6-alpine
GOLANGCI_LINT_VERSION := v2.15.0
SQLC_IMAGE := sqlc/sqlc:1.31.1
# Security pipeline scanners (docs/designs/devsecops.md). GITLEAKS_IMAGE's version is also the CI
# gitleaks action's GITLEAKS_VERSION; check-pins keeps the two equal.
TRIVY_IMAGE := aquasec/trivy:0.76.1
GITLEAKS_IMAGE := ghcr.io/gitleaks/gitleaks:v8.24.3
# make dast: ZAP, Nuclei and its templates release, and the curl image that drives the app.
ZAP_IMAGE := zaproxy/zap-stable:2.17.0
NUCLEI_IMAGE := projectdiscovery/nuclei:v3.11.1
NUCLEI_TEMPLATES := v10.5.1
CURL_IMAGE := curlimages/curl:8.22.0
export PG_IMAGE

VERSION ?= $(or $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//'),dev)

# Never let Go walk into web/node_modules (some npm packages ship .go files).
GO_PKGS := $(shell go list -e ./... | grep -v /node_modules/)
# golangci-lint takes directories, not import paths.
GO_DIRS := $(shell go list -e -f '{{.Dir}}' ./... | grep -v /node_modules/ | sed 's|^$(CURDIR)|.|')
# The packages with non-test files: go build refuses a test-only package (internal/guards).
GO_BUILD_PKGS := $(shell go list -e -f '{{if .GoFiles}}{{.ImportPath}}{{end}}' ./... | grep -v /node_modules/)

# CI installs golangci-lint with its action and sets GOLANGCI_LINT=golangci-lint.
GOLANGCI_LINT ?= bin/tools/golangci-lint-$(GOLANGCI_LINT_VERSION)/golangci-lint

# Unreachable functions `deadcode -test` may report; matched exactly, so it only falls. 0 since P1
# (serve uses clock.Real.Now, the one P0 entry).
DEADCODE_BUDGET := 0

# Packages make cover holds to the 90% statement floor (an owner decision; design doc item 2). It
# grows by phase: auth (P2); attributes, appointments, templates (P3); reminders (P4).
COVER_PKGS := ./internal/auth ./internal/attributes ./internal/appointments ./internal/templates ./internal/reminders
COVER_FLOOR := 90.0

# The dev Postgres container of make dev and make e2e (design doc item 2). The password is for this
# local container only.
DEV_PG_CONTAINER := open-customer-dev
DEV_PG_PORT ?= 5433
DEV_PG_PASSWORD := devpassword
# make e2e's database when E2E_DATABASE_URL is not set: a separate database on the dev container.
E2E_DB_NAME := open_customer_e2e
# E2E_DATABASE_URL as given by the environment or the command line, before make e2e sets its default.
E2E_GIVEN := $(E2E_DATABASE_URL)

# Starts the dev Postgres container, or reuses it, and waits until it answers (make dev, make e2e).
# The wait is over TCP: on a first start the image's init server answers on the socket only, then
# restarts.
define dev_pg_up
@if ! docker container inspect $(DEV_PG_CONTAINER) >/dev/null 2>&1; then \
	docker run -d --name $(DEV_PG_CONTAINER) -p $(DEV_PG_PORT):5432 \
		-e POSTGRES_USER=open_customer -e POSTGRES_DB=open_customer -e POSTGRES_PASSWORD=$(DEV_PG_PASSWORD) \
		$(PG_IMAGE) >/dev/null; \
else docker start $(DEV_PG_CONTAINER) >/dev/null; fi
@until docker exec $(DEV_PG_CONTAINER) pg_isready -q -h 127.0.0.1 -U open_customer -d open_customer; do sleep 1; done
endef

SHELL_FILES := $(shell find . -name '*.sh' -not -path './web/node_modules/*' -not -path './.git/*')

PNPM := cd web && pnpm

.PHONY: web-stub web build test vuln lint check-pins image node-check generate check-generated test-db cover dev e2e test-deploy \
	image-scan secrets test-sec dast

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

# Tests tagged db use testcontainers Postgres (PG_IMAGE, exported above); Docker must be running.
test-db: web-stub
	go test -tags db $(GO_PKGS)

# The db tests count too, so each package's profile includes them. A package under the floor fails
# the target and is named.
cover: web-stub
	@for pkg in $(COVER_PKGS); do \
		name=$$(basename "$$pkg"); \
		go test -tags db -coverprofile="coverage-$$name.out" "$$pkg"; \
		total=$$(go tool cover -func="coverage-$$name.out" | awk '/^total:/ { sub(/%/, "", $$NF); print $$NF }'); \
		echo "cover: $$pkg $$total% (floor $(COVER_FLOOR)%)"; \
		if ! awk -v t="$$total" -v f="$(COVER_FLOOR)" 'BEGIN { exit !(t != "" && t + 0 >= f + 0) }'; then \
			echo "cover: $$pkg is at $$total%, under the $(COVER_FLOOR)% floor" >&2; exit 1; \
		fi; \
	done

vuln: web-stub
	go tool govulncheck $(GO_PKGS)

$(GOLANGCI_LINT):
	GOBIN=$(CURDIR)/$(dir $@) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# The deadcode list is captured, then echoed to stderr: `tee /dev/stderr` reopens a redirected
# stderr file and truncates it (make lint > lint.log 2>&1 lost the golangci-lint result).
lint: web-stub node-check $(GOLANGCI_LINT)
	go mod tidy -diff
	$(GOLANGCI_LINT) run $(GO_DIRS)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(GO_BUILD_PKGS)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GO_BUILD_PKGS)
	@out=$$(go tool deadcode -test -tags db $(GO_PKGS)) || exit 1; \
	if [ -n "$$out" ]; then printf '%s\n' "$$out" >&2; fi; \
	n=$$(printf '%s' "$$out" | awk 'NF { c++ } END { print c + 0 }'); \
	if [ "$$n" -ne $(DEADCODE_BUDGET) ]; then \
		echo "deadcode: $$n unreachable functions, DEADCODE_BUDGET is $(DEADCODE_BUDGET) (remove the new one, or lower the budget)" >&2; exit 1; fi
	shellcheck $(SHELL_FILES)
	$(PNPM) run check
	$(PNPM) run lint

# Design doc item 2: oapi-codegen, openapi-typescript and sqlc (Docker image).
generate: web-stub node-check
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml
	$(PNPM) run gen:api
	docker run --rm -u $$(id -u):$$(id -g) -v $(CURDIR):/src -w /src $(SQLC_IMAGE) generate

# Fails on a modified or untracked generated file, not on unrelated uncommitted work.
check-generated: generate
	@out=$$(git status --porcelain -- internal/httpapi/gen internal/db/sqlc web/src/lib/api/schema.d.ts); \
	if [ -n "$$out" ]; then echo "generated files differ from the committed ones (run make generate and commit):" >&2; echo "$$out" >&2; exit 1; fi

# Design doc item 2 (P7): the backup image and deploy/tests/backup_test.sh against a throwaway
# Postgres, then deploy/tests/scripts_test.sh on the host; needs Docker, publishes no host port,
# and removes its oc-deploy-* containers.
test-deploy:
	PG_IMAGE=$(PG_IMAGE) scripts/test-deploy.sh

check-pins:
	PG_IMAGE=$(PG_IMAGE) GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) GITLEAKS_IMAGE=$(GITLEAKS_IMAGE) scripts/check-pins.sh

# Postgres in Docker (started or reused), serve on :8080 in the background, and Vite on :5173 in
# the foreground; the trap stops serve when Vite exits. serve runs as a built binary, not through
# `go run`: go run does not pass SIGTERM on, so the trap would leave the server running. There is
# no backup status file in dev, so the migration guard is off.
dev: web-stub node-check
	$(dev_pg_up)
	go build -o bin/dev/open-customer ./cmd/open-customer
	@PUBLIC_URL=http://localhost:5173 DB_HOST=localhost DB_PORT=$(DEV_PG_PORT) DB_PASSWORD=$(DEV_PG_PASSWORD) \
		VAPID_SUBJECT=mailto:dev@localhost ALLOW_MIGRATE_WITHOUT_BACKUP=true \
		bin/dev/open-customer serve & server=$$!; \
	trap 'kill $$server 2>/dev/null || true' EXIT; \
	$(PNPM) dev

# Design doc item 79: the built binary against an empty database, then Playwright WebKit. CI sets
# E2E_DATABASE_URL to its service container: that database is dropped and created again through the
# same server's postgres database, and no container is started. Otherwise the dev container is
# started or reused and $(E2E_DB_NAME) on it is created again.
e2e: build
ifeq ($(E2E_GIVEN),)
	$(dev_pg_up)
	docker exec $(DEV_PG_CONTAINER) dropdb -U open_customer --if-exists --force $(E2E_DB_NAME)
	docker exec $(DEV_PG_CONTAINER) createdb -U open_customer $(E2E_DB_NAME)
else
	@name=$$(node -e 'process.stdout.write(new URL(process.env.E2E_DATABASE_URL).pathname.slice(1))'); \
	admin=$$(node -e 'const u = new URL(process.env.E2E_DATABASE_URL); u.pathname = "/postgres"; process.stdout.write(u.href)'); \
	case "$$name" in *_e2e) ;; *) echo "make e2e: E2E_DATABASE_URL must name a *_e2e database" >&2; exit 1;; esac; \
	psql -q -v ON_ERROR_STOP=1 "$$admin" -c "DROP DATABASE IF EXISTS \"$$name\" WITH (FORCE)" -c "CREATE DATABASE \"$$name\""
endif
	$(PNPM) e2e
e2e: export E2E_DATABASE_URL := $(or $(E2E_GIVEN),postgres://open_customer:$(DEV_PG_PASSWORD)@localhost:$(DEV_PG_PORT)/$(E2E_DB_NAME))

# The app image and the backup image, both tagged $(VERSION) (make image-scan scans them).
image:
	docker build --pull --build-arg VERSION=$(VERSION) -t open-customer:$(VERSION) .
	docker build --pull -t open-customer-backup:$(VERSION) deploy/backup

# devsecops.md section 4: Trivy on open-customer:$(VERSION) and open-customer-backup:$(VERSION)
# (built by make image), $(PG_IMAGE) and the cloudflared image of deploy/compose.yaml. MODE=push
# blocks on the app and backup images only; MODE=release blocks on all four. PULL=1 pulls the
# released app and backup images (the ghcr names in deploy/compose.yaml) instead of the local
# build, for the weekly rescan. SARIF_DIR=<dir> also writes one SARIF file per image (CI).
MODE ?= push
PULL ?=
image-scan:
	TRIVY_IMAGE=$(TRIVY_IMAGE) VERSION=$(VERSION) PULL=$(PULL) scripts/security/image-scan.sh $(MODE)

# devsecops.md section 6: ZAP (API scan signed out and signed in, baseline) and Nuclei against
# open-customer:$(VERSION) (built by make image) on a private network, with no published port.
# Reports go to REPORT_DIR (git-ignored); NUCLEI_CACHE_DIR keeps the downloaded templates.
REPORT_DIR ?= dast-report
dast:
	IMAGE=open-customer:$(VERSION) PG_IMAGE=$(PG_IMAGE) ZAP_IMAGE=$(ZAP_IMAGE) NUCLEI_IMAGE=$(NUCLEI_IMAGE) \
		NUCLEI_TEMPLATES=$(NUCLEI_TEMPLATES) CURL_IMAGE=$(CURL_IMAGE) REPORT_DIR=$(REPORT_DIR) \
		scripts/security/dast.sh

# gitleaks over the whole git history, offline; reviewed false positives are in .gitleaksignore.
secrets:
	docker run --rm --name oc-secrets-gitleaks --network none -v "$(CURDIR)":/repo -w /repo \
		$(GITLEAKS_IMAGE) detect --redact --no-banner

# The scan wrappers' tests (scripts/security/tests/run.sh), with a fake docker.
test-sec:
	bash scripts/security/tests/run.sh
