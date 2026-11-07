---
paths:
  - 'api/**'
  - 'internal/httpapi/**'
  - 'web/src/lib/api/**'
---

# Rule: API

- **One typed contract.** Routes, request and response shapes, and error codes are defined once in
  the OpenAPI spec; Go gets the generated server interface, the web gets the generated
  `schema.d.ts`. No literal route string outside the contract and the web API client. Sensor:
  `make check-generated` + the compiler and svelte-check; a guard for literal routes is prose only
  until it lands.
- **Every request is parsed at the handler boundary** (`boundaries.md`): body, query, path params,
  headers. Sensor: `decodeJSON` with `DisallowUnknownFields` + generated param binding.
- **Responses are the generated types**, so a bug cannot leak an internal field. Handlers convert
  module structs to `gen` types; modules never import `gen`. Sensor: depguard (no domain package
  imports the generated package); contract tests.
- **Errors are codes, not messages.** A refusal returns an RFC 9457 problem with a stable `type` from
  the one list in the problem package; the web maps copy from the code to `es.json`. Never return a
  stack trace or a raw error message. Sensor: Semgrep `no-raw-error-to-client` (`make sast`: `http.Error`
  or a write to the response with an error value in it; the generated package is skipped).
- **Status codes tell the truth.** A failure never returns 2xx. An unknown path or method under
  `/api/` is a 404 problem, never `index.html`. Prose only; per-route tests.
- **Auth and input limits on every route by default** (body size, Origin check, login rate limit).
  A route without auth says why in a comment (setup status, healthz, readyz, manifest, icons).
  Sensor: gosec; reviewer.
- **Writes commit before the response.** Every non-GET handler that reads or writes the database runs its
  module call in `db.WithTx` and commits before it writes the body, so a failed commit is never
  shown as saved. Sensor: Semgrep `non-get-handler-in-withtx` (`make sast`: a handler whose name starts
  with a non-GET verb never uses `s.pool`; guard `TestSemgrepWriteVerbsMatchTheSpec` fails when the
  rule's verb list stops matching the non-GET operations of `api/openapi.yaml`); reviewer check for
  the rest.
- **Handlers stay thin**: parse → call the module → map the result. Logic lives in the module.
  Sensor: depguard (module rules); reviewer.
- **Security headers on every response** (`X-Content-Type-Options`, `Referrer-Policy`, the header
  CSP, HSTS only for an https `PUBLIC_URL`) come from the one middleware `securityHeaders`; scripts
  and styles are allowed by `kit.csp` in `web/vite.config.ts`, never by a handler. Sensor:
  `TestSecurityHeadersOnEveryResponse`; `scripts/security/csp-check.sh` (run by `make build` on
  the built `index.html`'s `<meta>` CSP); `make dast` (ZAP, Nuclei) in CI.
- **Secrets come from the environment, never from code or logs.** Sensor: gosec + gitleaks in CI.
