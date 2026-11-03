# open-customer

A self-hosted app for one small service business (a nail studio, a barber, a beauty salon) to keep
its clients, appointments and reminder messages. The owner sends reminders by hand through
WhatsApp; the app tells her who is due and writes the message. It is one Go binary that serves a
SvelteKit PWA, with PostgreSQL, meant to run on a home PC behind a Cloudflare Tunnel.

- Install and operate: deploy/README.md (written in P7).
- Architecture and build plan: [docs/designs/release-1.md](docs/designs/release-1.md).
- License: not chosen yet.

## Development

Needs Go (the `toolchain` in `go.mod`), Node and pnpm (the exact versions in `web/package.json`;
with fnm, `fnm use` in `web/`), Docker, shellcheck and make.

Run every Go command through `make`. `web/embed.go` embeds `web/build`, which is not in git, so a
plain `go build` or `go test` on a clean checkout fails until `make web-stub` (or `make web`) has
run.

| Command | What it does |
| --- | --- |
| `make check-pins` | Image and tool versions agree across the Makefile, `go.mod`, `web/package.json`, the Dockerfile and CI |
| `make lint` | `go mod tidy -diff`, golangci-lint, deadcode, shellcheck, svelte-check, eslint, prettier |
| `make test` | Go unit tests and Vitest |
| `make generate` | oapi-codegen, sqlc (Docker image) and openapi-typescript |
| `make check-generated` | `generate`, then fails if a generated file differs from the committed one |
| `make test-db` | Go tests tagged `db` against a testcontainers Postgres (needs Docker) |
| `make vuln` | govulncheck |
| `make build` | The web build, then `bin/open-customer` |
| `make dev` | Postgres in Docker (container `open-customer-dev`, host port `DEV_PG_PORT`, default 5433), `serve` on :8080 and Vite on :5173 |
| `make image` | The app Docker image |

The gate before any change is done: `make check-pins lint test vuln build check-generated test-db`.
Later phases add targets (`cover`, `e2e`); the design doc's "Build order" says when.

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
