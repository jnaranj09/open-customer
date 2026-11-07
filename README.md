# open-customer

A self-hosted app for one small service business (a nail studio, a barber, a beauty salon) to keep
its clients, appointments and reminder messages. The owner sends reminders by hand through
WhatsApp; the app tells her who is due and writes the message. It is one Go binary that serves a
SvelteKit PWA, with PostgreSQL, meant to run on a home PC behind a Cloudflare Tunnel.

- Install and operate: [deploy/README.md](deploy/README.md).
- Architecture and build plan: [docs/designs/release-1.md](docs/designs/release-1.md).
- Architecture decisions (ADR index, Spanish): [docs/sad.md](docs/sad.md).
- Product requirements (Spanish): [docs/prd.md](docs/prd.md).
- License: not chosen yet.

## Development

Needs Go (the `toolchain` in `go.mod`), Node and pnpm (the exact versions in `web/package.json`;
with fnm, `fnm use` in `web/`), Docker, shellcheck and make.

Run every Go command through `make`. `web/embed.go` embeds `web/build`, which is not in git, so a
plain `go build` or `go test` on a clean checkout fails until `make web-stub` (or `make web`) has
run.

| Command | What it does |
| --- | --- |
| `make check-pins` | Pins agree across the Makefile, `security/tools.Dockerfile` (where the tool and Postgres images are pinned), `go.mod`, `web/package.json`, the Dockerfiles, `deploy/compose.yaml` and CI (the Postgres image's digest included), and the cosign version in `deploy/scripts/lib.sh` and the release workflow |
| `make lint` | `go mod tidy -diff`, golangci-lint, `go build` for darwin and windows, deadcode, shellcheck, svelte-check, eslint, prettier, knip, jscpd |
| `make test` | Go unit tests and Vitest |
| `make generate` | oapi-codegen, sqlc (Docker image) and openapi-typescript |
| `make check-generated` | `generate`, then fails if a generated file differs from the committed one |
| `make test-db` | Go tests tagged `db` against a testcontainers Postgres (needs Docker) |
| `make cover` | The `db` tests with coverage for each package in `COVER_PKGS`; fails when one is under 90% |
| `make test-deploy` | Builds the backup image and runs `deploy/tests/backup_test.sh` in it against a throwaway Postgres on a private Docker network (needs Docker; publishes no host port; removes its `oc-deploy-*` containers and network), then `deploy/tests/scripts_test.sh` on the host (the operator scripts against a fake `docker`) |
| `make vuln` | govulncheck |
| `make build` | The web build, then `bin/open-customer` |
| `make dev` | Postgres in Docker (container `open-customer-dev`, host port `DEV_PG_PORT`, default 5433), `serve` on :8080 and Vite on :5173 |
| `make image` | The app and backup Docker images, tagged with the version |
| `make image-scan` | Trivy on the two `make image` images and the pinned postgres and cloudflared images (needs Docker and network). `MODE=push` (the default) fails only on the app and backup images; `MODE=release` fails on any of the four |
| `make dast` | ZAP (API scan signed out and signed in, baseline) and Nuclei against the `make image` app image and a throwaway Postgres on a private Docker network (needs Docker and network; publishes no host port; removes its `oc-dast-*` containers and network). A made-up owner is set up and signed in first; reports go to `dast-report/` |
| `make scan-live URL=https://…` | A manual scan of your own deployment (needs Docker, network, bash 4, `jq` and coreutils; no key; never runs in CI): the security headers, the ZAP baseline (passive) and Nuclei (non-intrusive active probes), signed out. Non-zero means not clean; the script's own code (1 findings, 2 bad arguments) is in make's `Error N` line. The allow-list is your git-ignored `deploy/.scan-live.allow`; reports go to `security/reports/scan-live-<UTC timestamp>/`. See [deploy/README.md](deploy/README.md) section 14 |
| `make secrets` | gitleaks over the whole git history (needs Docker); reviewed false positives are in `.gitleaksignore` |
| `make test-sec` | The scan wrappers', release-sign's and check-pins' tests (`scripts/security/tests/run.sh`, against a fake `docker` and `cosign`) |
| `make release-sign IMAGE=<repository>:<tag>` | Release workflow only (needs the registry login, cosign and the job's OIDC token; not in the gate): pushes the image, takes the digest from the push, writes a CycloneDX SBOM with Trivy, then signs the digest keylessly with cosign and attests the SBOM |
| `make config-scan` | Trivy config on the two Dockerfiles and zizmor on `.github/workflows`, offline (needs Docker). Fails on a high or critical Dockerfile finding or a medium or high workflow finding |
| `make sast` | Semgrep with only the rules written in `security/semgrep`, offline (needs Docker): first the rules' fixtures (`semgrep --test`), then the scan of the repo. Any finding fails |
| `make fuzz` | The four Go fuzz targets (JSON body decoding, multipart uploads, template parsing, the backup status file), one after another, each for `FUZZTIME` (default `60s`). Any crash fails. Not in the gate; CI runs it weekly with `FUZZTIME=30s`; `make test` runs their seeds |
| `make e2e` | `make build`, a fresh e2e database (`open_customer_e2e` on the `make dev` container, or the database `E2E_DATABASE_URL` names, which must end in `_e2e`), then Playwright WebKit against the binary: the CUJ specs and the UI probe (axe and layout checks on every route at phone and desktop size). Needs the Playwright WebKit browser and its host libraries, port 8080 free, and Docker (local) or `psql` and `node` (when `E2E_DATABASE_URL` is set) |

The gate before any change is done: `make check-pins lint test vuln build check-generated test-db cover test-deploy secrets test-sec config-scan sast`.
For a change to an image, a scan or what the app serves, also `make image image-scan dast`.
The gate leaves out `make e2e`. CI runs it after `make build` (design doc item 8); locally it runs
by hand.

The binary's own subcommands (`serve`, `reset-password <username>`, `setup-link`, `healthcheck`,
`version`) are described in the design doc's "Process model" and item 19.

## Release checklist

Run before each release tag (the first time: before `v1.0.0`). Each item is behaviour-neutral, in
its own commit, with the full gate green.

1. **Duplicates.** Read the `dupl` (Go) and jscpd (web) reports; merge real copies, leave
   look-alikes.
2. **Dead code.** Remove what `deadcode`, `unused` and knip report, or write why it stays.
3. **Slow tests.** `go test -json` and the Vitest report: any test over 1 s gets a reason or a fix.
4. **Mutation testing.** Go: gremlins on `internal/reminders`, `internal/templates`,
   `internal/attributes`, `internal/appointments` and `internal/auth`. Web: StrykerJS with the
   Vitest runner on `web/src/lib/api/client.ts`, `web/src/lib/cache/` and `web/src/lib/search.ts`.
   Every surviving mutant ends as a new test that kills it or a one-line reason in the commit.
5. Record the date and the results below.

| Date | Release | Notes |
| --- | --- | --- |
| | | |
