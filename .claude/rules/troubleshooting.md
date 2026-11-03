---
paths:
  - 'cmd/**'
  - 'internal/**'
  - 'api/**'
  - 'web/src/**'
  - 'deploy/**'
  - '**/*_test.go'
  - 'web/tests/**'
---

# Rule: troubleshooting (characterize first, theorize second)

A symptom in one layer is easy to blame on another. Before reading code, answering a theory, or
fanning out subagents, answer these three (a 30-second manual repro).

- **Is the data present?** Check the row in Postgres, then the `/bootstrap` response, then the Dexie
  table. Present in the DB but missing on the phone ⇒ refresh / cache / render. Missing in the DB ⇒
  handler / module / transaction. Prose only.
- **Does it depend on a sibling?** Removing another item, request or process fixes it ⇒ shared
  resource, ordering or layout (another appointment in the overlap check, another item in the due
  list). Independent ⇒ the component's own lifecycle. Prose only.
- **Is the log empty?** Every contained failure logs a `code=`. Empty log + wrong behaviour ⇒ logic or
  presentation. A `code=` present ⇒ follow it to its layer. Prose only.
- **Reproduce with the real thing before writing a scripted diagnostic.** A manual repro beats a
  script that cannot run the real dependency (real Postgres via `make dev`, Safari's iPhone view).
  Prose only.
- **Dev-only and prod-only modes differ.** `make dev` serves the web app from Vite with a proxy and
  no service worker cache; the built binary serves the embedded build behind the service worker, and
  production adds the Cloudflare Tunnel and the business time zone. Check which mode the bug needs
  before chasing it. Prose only.
- **Add a row below for every bug that took more than one wrong theory**, with the cheapest test that
  would have found it. Prose only.

## Symptom → suspect → cheapest test

| Symptom | Suspect | Test |
| --- | --- | --- |
| | | |
