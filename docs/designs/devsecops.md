# open-customer — Security pipeline

Status: built (2026-11-07). The owner approved it on 2026-11-06. Every part exists and ran locally
except what the TASKS.md "Security pipeline exit" entry lists as NOT RUN: the first real keyless
signature (it needs a `v*` tag), the live scan against a real deployment, and the weekly workflow's
first scheduled run.

## Intent

Protect the app, first. Then give the repo a real security pipeline that any self-hoster can run.
Learning the tools is a minor, third goal.

The owner is not a security expert, so the pipeline must cover every gap, not only the ones a tool
happens to find. The way to do that is a threat model: a list of every way the app can be attacked
or lose data, where each threat names the check that covers it, or a written reason it is accepted.
The tools serve the threat model, not the other way round. This is the repo's existing rule ("every
rule names its sensor") applied to security.

## Goals

1. Every threat to the app has a check that turns red, or an accepted-risk line with its reason.
2. A high or critical finding with a fix blocks a merge.
3. A new vulnerability in an image that already shipped is found within a week, without anyone
   looking.
4. Anyone who deploys the app can scan their own live deployment with one command.
5. Nothing in the repo or in GitHub holds a credential for any outside service.

## Non-goals

- **The host machine.** The OS, the disk, the Docker daemon and `deploy/.env` at rest are the
  deployer's job. The threat model lists them as out of scope.
- **Cloudflare settings.** The repo stays generic: it must not depend on Cloudflare. A Cloudflare
  check, if any, lives outside the repo. What Cloudflare does is still seen from outside by the
  live scan (below), like any other front proxy.
- **Scheduling and alerting for the live scan.** Each deployer picks their own cron, email service
  or anything else. The repo gives an exit code and a report file.
- **Runtime monitoring (Falco, Tetragon).** Both need the host, which is out of scope. Later.

## Anti-goals (a failure even if it works)

- Any API key, token or password for an outside service stored in the repo, in a GitHub secret or in
  a GitHub variable, even read-only. CI uses only what GitHub gives every run (`GITHUB_TOKEN`, OIDC).
- A scan of a real deployment running in GitHub. On a public repo the Actions logs are public, so it
  would publish the URL and its weak points.
- A check that skips or passes silently when its tool is missing or its download fails.
- An allow-list entry with no reason or no expiry date, or a list that grows to get green.

## Constraints from the repo

- `CLAUDE.md` and `.claude/rules/*` apply. Every new check names its sensor; allow-lists only
  shrink; a check that could not run is reported as NOT RUN.
- Every tool and image is pinned in one place and kept equal by `make check-pins` (design doc
  "Pins"). Dependabot bumps what it can.
- `CLAUDE.md` names `docs/designs/release-1.md` as the architecture source for the app and this
  document for the security pipeline.
- CI already builds the app image and can start it with a throwaway Postgres. The dynamic scans
  run against that (section 6), so there is no staging environment.

## What gets built

### 1. Threat model (`docs/threat-model.md`)

- One table. Each row: the asset (client data, photos, the session, the backups, the images, the
  build), the threat, the entry point, and either the check that covers it (a test name, a guard, a
  scan rule) or `accepted:` with the reason.
- Seeded from release-1.md "Security summary", "Accepted risks" and "Handled risks", and from the
  attack surface: the public URL behind any proxy, login and setup, the session cookie, uploads,
  the API, the backup and restore scripts, the images, CI and the release workflow, dependencies.
- Out-of-scope items (host, Cloudflare dashboard) are rows too, marked `out of scope:` with who owns
  them, so the list is visibly complete.
- The "Covered by" cell holds typed references, separated by `; `: `test:<GoTestName>` (a `Test`
  or `Fuzz` function some `_test.go` declares, with one `*testing.T` or `*testing.F` parameter), `file:<repo path>`, `make:<target>`,
  `ci:<workflow>/<job>`, `accepted: <reason>`, `out of scope: <who owns it>`, `manual: <what and how
  often>` (GitHub settings). There is no `gap:` type: it was allowed while the pipeline was built
  and the guard now refuses it.
  The allow-list is not a check, so it is never a reference.
- The guard `TestThreatModel` (`internal/guards`, run by `make test`) fails on a row with an empty
  "Covered by", a reference of another type (`gap:` included), a test no `_test.go` declares, a file that does not
  exist, a make target the Makefile lacks, a CI job the workflow lacks, or a duplicate ID.
- The one-time manual audit (section 8) adds rows.

### 2. Code scan (Semgrep)

- `make sast` (`scripts/security/sast.sh`, in the gate) runs Semgrep with **only rules written in
  this repo** (decision D2): `security/semgrep/<rule id>.yaml`, one rule per file, each with an
  annotated fixture in `security/semgrep/testdata/` (`ruleid:` lines the rule must report, `ok:`
  lines it must not; the folder is named `testdata` so the Go tools and linters skip the Go
  fixtures). The Semgrep Rules License forbids redistributing the registry rules, and fetching
  them at run time is unpinned, so none is used; a licensed ruleset is a later owner decision.
  Generic Go security stays with gosec in golangci-lint; Semgrep carries the repo-specific rules and
  the TypeScript and Svelte ones. Overlap with the existing linters is on purpose, as defence in
  depth: `no-dynamic-sql` overlaps gosec G201 and G202, and `svelte-no-html-tag` overlaps eslint
  `svelte/no-at-html-tags`. The linter stays the first sensor; the Semgrep rule is the second.
- The rules, each with the threat it closes (`docs/threat-model.md`):
  - `no-slog-of-request-body` (T22): a `slog` or standard `log` call with `r.Body`, `r.Form`, a
    value `decodeJSON`, `decodePatch` or `json.NewDecoder(r.Body).Decode` filled (or a field of it)
    or the error they return, until that error variable is given another call's error. It replaced
    the "log a code, never the value" prose in `boundaries.md` for those values.
  - `no-raw-error-to-client` (T23): an `.Error()` call (any variable), or an error-named value,
    in `http.Error`, a write to the response, `writeJSON`, `json.NewEncoder(w).Encode`,
    `problem.Field` or a `problem.FieldError` message. The generated `internal/httpapi/gen` is
    skipped (its default handler is replaced).
  - `no-dynamic-sql` (T27): a `fmt.Sprintf`, `+` or `String()` result reaching a query argument
    of `Exec`, `Query`, `QueryRow` or `Prepare`.
  - `non-get-handler-in-withtx` (T96): a handler on `*Server` whose name starts with a non-GET verb
    uses `s.pool`. Semgrep cannot see the HTTP method, so the rule lists the verbs of the non-GET
    `operationId`s; a new verb is added to the rule and its fixture. A non-GET handler that
    reaches the pool another way is left to the reviewer. The guard
    `TestSemgrepWriteVerbsMatchTheSpec` (`internal/guards`) reads the rule's verb regex and the
    non-GET `operationId`s of `api/openapi.yaml`, and fails on a non-GET one the regex does not
    match or a GET one it does, so a new verb cannot slip by.
  - `ts-no-inner-html` (T28): `innerHTML`, `outerHTML` (also by `el['innerHTML']` and
    `Object.assign`), `insertAdjacentHTML`, `createContextualFragment`, `setHTMLUnsafe`,
    `parseHTMLUnsafe` and `document.write` in TypeScript and JavaScript.
  - `svelte-no-html-tag` (T28): `{@html` in `.svelte` files. Semgrep 1.182 has no Svelte parser,
    so this rule reads the file as text (`generic`).
  - `ts-no-eval` (T29): `eval` (also `window.eval`, `self['eval']`, `(0, eval)(x)`), `new Function`,
    `Function(...)` and a string handed to `setTimeout` or `setInterval`.
- Each rule was seen red once on real code (a forbidden call added, `make sast` red, removed).
- The wrapper runs `semgrep --test` first (every rule file needs a fixture with a `ruleid:` line,
  and every annotation must match), then the scan, both in the pinned image with `--network none`,
  `--metrics=off` and `--disable-version-check`. A scan that checked no Go, TypeScript or Svelte
  file, a report with errors and a tool error are red. `.semgrepignore` lists what is skipped: tests
  (`*_test.go`, `*.test.ts`, `web/tests/`) and the rule fixtures, which hold every forbidden call on
  purpose. Semgrep reads the files git lists, so a new file is scanned before it is added; in CI
  all are tracked. Limit: Semgrep does not report a parse error in a Go file (it does in
  TypeScript), so a Go file that does not parse is skipped silently; `make lint` and `make build`
  catch it.
- An allow-list entry for Semgrep (`{"tool": "semgrep", "id": "<rule id>"}`) is the plain rule id,
  which `sast.sh` turns into `--exclude-rule security.semgrep.<id>`; an id that is no rule of
  `security/semgrep` is refused. A rule cannot be narrowed to one file by the allow-list: narrow the
  rule's `paths:` instead.
- CI uploads the SARIF file to the Security tab from job `security` (category `semgrep`).

### 3. Config scan (Dockerfiles, compose, workflows)

- `make config-scan` (`scripts/security/config-scan.sh`, in the gate). Neither Checkov nor Trivy's
  config scanner reads Compose files (tool research, 2026-11-06), so each config kind has its own
  sensor and Checkov is not used:
  - **Dockerfiles** (`Dockerfile`, `deploy/backup/Dockerfile`): Trivy config, with the checks built
    into the pinned Trivy image (`--skip-check-update`, no bundle fetched at run time) and no
    network. A high or critical finding blocks; medium and low are not reported.
  - **Workflows** (`.github/workflows/`): zizmor `--offline` (unpinned actions, too-wide
    permissions, script injection, credential persistence, cache poisoning), no network. A medium
    or high finding blocks (exit 13, 14); informational and low are printed only (exit 11, 12). CI
    job `security` uploads its SARIF.
  - **`deploy/compose.yaml`**: the Go guard `TestComposeHardening` (`internal/guards`, run by
    `make test`).
- The fixes it forced: compose hardening (every service `no-new-privileges`, `cap_drop: [ALL]`,
  `read_only: true` with a tmpfs where it writes, a memory limit; `db` runs as the image's `postgres`
  user and needs no capability; `backup` and `restore` add back only the capabilities each was
  seen failing without), `USER 65532:65532` in the app image on a distroless static `nonroot`
  base with no `RUN` in its final stage, so no shell (guard `TestAppImage`),
  `persist-credentials: false` on every checkout, no Go cache in the release workflow, and
  `packages: write` and `id-token: write` on the release job only.
- Supply-chain pins: every `uses:` is a full commit SHA with its `# vX.Y.Z` and every image a
  `<tag>@sha256:<index digest>` (every Dockerfile, compose, CI services and job containers, the
  deploy scripts), checked by the guard `TestSupplyChainPins`. The same guard refuses a
  `docker/setup-buildx-action` step that would pull an unpinned BuildKit image: it must use
  `driver: docker` (the runner's own engine; the release workflow does) or name the BuildKit image
  by digest in `driver-opts`. The tool and Postgres images live in
  `security/tools.Dockerfile` (never built; `make config-scan` lists it as such), which the Makefile
  reads and Dependabot's docker ecosystem bumps; `make check-pins` keeps the copies of the Postgres
  pin, digest included, equal. Dependabot bumps the SHA or digest and the version together; for
  postgres, node and golang it proposes only a new digest of the same tag, and a version bump is
  made by hand. `make config-scan` fails on a Dockerfile it neither scans nor lists as never
  built.

### 4. Image scan and supply chain (Trivy)

- On every push: build the app and backup images and scan them, plus the pinned `postgres` and
  `cloudflared` images from `deploy/compose.yaml`.
- On every push, in job `gate`: `make deps-scan` (`scripts/security/deps-scan.sh`), Trivy fs on
  `web/pnpm-lock.yaml` (development dependencies included) and `go.mod`, each mounted alone, with
  the same database cache handling as the image scan (no database is exit 1, red). Its JSON report
  must list both manifests, and an exit that disagrees with the report is red. Tests: `make
  test-sec` (`deps-scan:` cases).
- In the release workflow: the first step, `scripts/security/release-on-main.sh`, refuses a tag
  whose commit is not an ancestor of `main` (fail closed: a missing SHA, a shallow clone and a
  failed fetch are red; `make test-sec` covers it). A commit that reached `main` before branch
  protection was applied has not necessarily passed CI; after it, every commit on `main` came
  through a green PR. Then scan before push; a blocking finding stops the release.
- Weekly scheduled CI run: scan the images of the latest release tag only (deployers upgrade to the
  latest): the newest `vX.Y.Z` tag by version order, pre-release tags (`v1.3.0-rc1`) skipped
  (`scripts/security/latest-release.sh`; `make test-sec` covers it), so an rc never hides the
  release deployers run. A finding that meets the blocking policy fails the run, and GitHub emails the owner about
  a failed scheduled run with no setup. Until the first release tag exists, the job writes "NOT RUN: no
  release tag" in its summary and passes. My call. Limit: in a public repo GitHub turns off a
  schedule after 60 days with no repository activity, with no red run and no email, so the owner
  checks the Actions tab monthly and re-enables `security-weekly` if it was turned off
  (`docs/threat-model.md` T71 and "GitHub settings"). No keepalive commits.
- My call: the release also writes an SBOM and signs the images keylessly with cosign through
  GitHub OIDC. Keyless signing stores no key, so it keeps anti-goal 1. Built as
  `make release-sign IMAGE=<repository>:<tag>` (`scripts/security/release-sign.sh`), run by the release
  job for the app and the backup image after the scan: `docker push`; the digest from the push's
  `<tag>: digest: sha256:… size: …` line, which the local image's `RepoDigests` must also hold (no
  digest, two digests or a mismatch is red before cosign runs); a CycloneDX SBOM of that image
  with Trivy (the `TRIVY_IMAGE` pin, `--network none`); `cosign sign --yes` and `cosign attest
  --yes --type cyclonedx` on `<repository>@<digest>`, never the tag. cosign comes from
  `sigstore/cosign-installer` with `cosign-release`, and the job has `id-token: write`. Tests:
  `make test-sec` (`release-sign:` cases, fake `docker` and `cosign`). A real keyless sign needs
  the job's OIDC token, so the first `v*` tag is its first test.
- My call: `deploy/scripts/upgrade.sh` verifies the signature before it pulls, running cosign from
  its pinned image (no install on the deployer's machine). A bad or missing signature stops the
  upgrade with nothing changed; `make test-deploy` gets a test for it. Built as `verify_image` in
  `deploy/scripts/lib.sh`: `cosign verify <image>:<version> --certificate-identity-regexp
  "$SIGNER_IDENTITY_REGEXP" --certificate-oidc-issuer "$SIGNER_ISSUER"
  --certificate-github-workflow-repository "$SIGNER_REPOSITORY"
  --certificate-github-workflow-trigger "$SIGNER_TRIGGER"` (decision D11: this repository's
  `release.yml` on a `refs/tags/v` ref, issued by `https://token.actions.githubusercontent.com`,
  run in this repository on a push), for the app and the backup image, after the version check and
  before the pull and the pre-upgrade backup. There is no flag to skip it. It keeps the one digest
  cosign verified (`critical.image.docker-manifest-digest`; none or two distinct ones is red), and
  after `compose pull` each pulled image's `RepoDigests` must hold `<image>@<that digest>`, or the
  upgrade stops before the backup with `.env` unchanged: a tag moved between the check and the pull
  cannot be run. The tag is never resolved again, and `compose.yaml` sets no `pull_policy` that
  pulls by itself (guard `TestComposeHardening`), so `up -d` runs the checked local images. The
  same steps are `pull_verified <version>` in `lib.sh`, the one way a script brings release images
  onto the PC: `upgrade.sh` (step 2), `deploy/scripts/pull.sh` (the first install, before `up -d`),
  and, through `ensure_release_images` (no network when both images are local), `restore.sh` (for
  `.env`'s version before it reads the snapshot, and for the snapshot's version before it stops
  anything) and `backup-now.sh`. A local image under a release name is trusted: the scripts put
  one there only through `pull_verified`, which removes both tags when its pull or digest check
  fails (a tag it cannot remove gets a loud message with the `docker image rm` command);
  `make image` tags `open-customer:<version>`, not the release names. A fork changes
  `SIGNER_IDENTITY_REGEXP` and `SIGNER_REPOSITORY` (`deploy/README.md` section 9). `COSIGN_IMAGE`
  lives in `lib.sh`, because deployers have `deploy/` and not the Makefile; `make check-pins` keeps
  its version equal to `cosign-release` in the release workflow, and Dependabot reads neither, so
  a cosign bump is made by hand, both copies in one commit.

### 5. Authorization and business-rule tests

- Mostly there already: `TestPublicOperationsAreExactlyTheSetupLoginAndPushKeyOnes` reads the spec
  and fails if any other route is public; Origin, the must-change-password gate and the login rate
  limit have tests. Release 1 has one user, so cross-user ownership tests do not apply yet.
- The threat model maps each auth threat to its test; any threat without one gets a new Go test
  here.

### 6. Running-app scan in CI (ZAP + Nuclei)

- Against the app image `make image` builds, in CI job `security` (`make dast`,
  `scripts/security/dast.sh`): the app and a throwaway Postgres on a private Docker network with no
  published port (host :8080 stays free for `make e2e`). The scanners run in the app container's
  network namespace and reach it as `http://localhost:8080`, its `PUBLIC_URL` (the app takes plain
  http only on localhost). The ZAP rules file, the ZAP report checks, the Nuclei templates fetch
  and the Nuclei run are in `scripts/security/lib.sh`, which `scan-live.sh` (section 9) shares.
- ZAP API scan driven by `api/openapi.yaml`, unauthenticated and authenticated: a made-up owner is
  created through `setup-link` and `POST /setup` and logs in, and `GET /me` must answer 200 with
  her cookie before any scan runs (and again after the signed-in pass, which leaves logout out).
  Plus a ZAP baseline (passive) scan of `/`.
- Nuclei against the same app, with a pinned templates release (its tarball's sha256 pinned too).
  Out-of-band checks are not covered: the network is internal, so the interactsh templates are
  turned off (`-ni`).
- The fix it forced: security headers on every response (`Content-Security-Policy` including
  `frame-ancestors`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, and
  `Strict-Transport-Security` only when `PUBLIC_URL` is https), set by the middleware, with the
  script and style sources in a `<meta>` CSP that SvelteKit writes with script hashes. Go test:
  `TestSecurityHeadersOnEveryResponse`. The `<meta>` has its own check: `make web` (so `make build`
  and `make e2e`) runs `scripts/security/csp-check.sh` on the built `web/build/index.html` right
  after `pnpm build`. It is red unless the page has exactly one CSP `<meta>` with `default-src
  'self'`, `script-src` of `'self'` and sha256 hashes only (no `'unsafe-inline'`, `'unsafe-eval'`
  or host; no `script-src-elem` or `script-src-attr`), no `object-src` other than `'none'`, no
  `frame-ancestors` (ignored in a `<meta>`; the header sets it), and every inline `<script>`
  hashes to a listed source. The web-stub has no `<meta>`, so the check refuses it rather than
  passing it. Tests: `make test-sec` (`csp-check:` cases).

### 7. Fuzz tests

- Go native fuzz tests on the parsers that take outside input, one per seam:
  - `FuzzDecodeJSON` (`internal/httpapi`): `decodeJSON` and `decodePatch`, the one parse site of
    every JSON route. A route cannot be driven without a database (the session check is first), so
    the fuzzer drives the decoders. A body is decoded (no U+0000 in it, nothing written) or refused
    with a 4xx problem; never a panic or a 5xx.
  - `FuzzMultipartUpload` (`internal/httpapi`): `readIconParts` and `readMediaParts`, the multipart
    parse before any database call, for any content type and body. Read, or a 415, 413 or 422
    problem; never a panic or a 5xx.
  - `FuzzParseTemplate` (`internal/templates`): `Parse` and `Render` never panic; the error is a
    validation on field `body`; positions are inside the body; a parsed body rendered with a value
    for every variable parses again with no variable.
  - `FuzzBackupStatusRead` (`internal/platform/backupstatus`): `Read` returns a status or
    `ErrInvalid`, never a panic, and its error never names the path.
- `make test` runs each seed (the `f.Add` calls and `testdata/fuzz/<Name>/`) as a normal test.
  `make fuzz FUZZTIME=60s` runs each target for that long, one after another. It is not in the
  gate; `security-weekly.yml` job `fuzz` runs `make fuzz FUZZTIME=30s` every week (a step timeout
  of 20 minutes, a crash fails the run and its input is uploaded as an artifact), so the rows that
  cite `make:fuzz` also cite `ci:security-weekly.yml/fuzz`. A crash found by fuzzing becomes a seed
  and a fix.

### 8. Manual audit (once, then on big changes)

- A one-time security review of the code for logic bugs no scanner finds (the `security-audit`
  skill fits). Findings become threat-model rows and fixes.

### 9. Live scan (`make scan-live URL=…`)

- Runs on the deployer's own machine against any deployment, behind any proxy. The tools run in
  Docker; the host needs Docker, the repository checkout, `make`, bash 4, `jq` and coreutils
  (checked first: a missing one exits 2 naming it). Uses no key and never logs in. `scripts/security/scan-live.sh`, shared with `dast.sh` through
  `scripts/security/lib.sh` (the ZAP rules file, the ZAP report checks, the pinned Nuclei templates
  and the Nuclei run).
- A `curl` header check (the fixed headers of section 6 with the app's exact values, plus HSTS on
  https; a missing header or a wrong value is a finding), the ZAP baseline (passive), and Nuclei
  (non-intrusive but active probes, default-credential checks on other products' paths among them;
  no intrusive, dos or fuzz templates; its `ssl` templates cover the TLS of an https URL).
- The script exits 0 clean, 1 on any finding or a tool that failed to run, 2 on bad arguments.
  Through `make` any failure is exit 2 and the script's code is in make's `Error N` line, so a
  schedule reads non-zero as "not clean". The report goes to
  `security/reports/scan-live-<UTC timestamp>/` (git-ignored); an existing non-empty `REPORT_DIR` is
  refused, never cleared. `deploy/README.md` documents the exit code, the report and how to
  schedule it with cron.
- Never runs in GitHub: it exits 2 when `GITHUB_ACTIONS=true`, and no workflow calls it.
- Its allow-list is a local file the deployer owns, kept out of git (`deploy/.scan-live.allow`,
  git-ignored), because what a proxy adds or strips differs per deployment. My call. Each line is
  `<tool> <id> # <reason>` for `zap` (a rule number, or `<rule>@<url regex>`), `nuclei` (a template
  id) or `header` (a header name); a malformed line is refused with its line number, and a scoped
  `zap` regex gets the checks of `TestSecurityAllowList` (ends in `$`, compiles, matches none of the
  probe URLs on localhost or on the scanned origin). There is no
  expiry date: the file is the deployer's own, not the repo's.
- The app answers plain http only on localhost, so the URL must be https. `NETWORK=` (a Docker
  network or `container:<name>`) is a test hook: the tool containers join it and `http://localhost`
  is accepted. It is how the scan was run once against a throwaway local stack; a real deployment
  is scanned by its owner.

## Blocking policy

- **CVE scanners** (Trivy on images and dependencies): high or critical with a fixed version
  available blocks. Medium and low are reported, not blocking. The dependency scan is
  `make deps-scan` (Trivy fs on `web/pnpm-lock.yaml`, development dependencies included, and on
  `go.mod`; `--ignore-unfixed`, so an advisory with no fix never blocks; `trivy` allow ids
  honoured), in CI job `gate`. It replaced `pnpm audit --audit-level high`, which has no "fix
  available" filter and no allow path. `make vuln` (govulncheck) stays the Go check of what the
  code calls.
- **Every other tool** (Semgrep, Nuclei, the compose and threat-model guards): every finding
  blocks unless it is allow-listed. Their severity scales differ and have no "fix available", and a
  missing header or `cap_drop` must go red to be fixed. My call. The config scan blocks from a set
  severity instead (builder's call, section 3): Trivy config on high and critical, zizmor on medium
  and high; zizmor's informational and low findings are printed only.
- **ZAP** blocks on its High and Medium rules and on the header rules 10021 (`nosniff`) and 10035
  (HSTS), all FAIL in `security/zap-rules.tsv`. Its Low and Informational rules only WARN,
  10011 (cookie without `Secure`, the scan is plain http) and 10116 (ZAP out of date) included
  (controller ruling, recorded in that file). `make dast` is also red on a 5xx answer in a ZAP
  report and on an alert from a rule the file does not list. Unless allow-listed.
- **Third-party images** (`postgres`, `cloudflared`): on push and PR they report only, so an
  upstream CVE with no new image tag yet cannot turn every unrelated push red. The release and the
  weekly run block on them. Our own images (app, backup) block on push. My call.
- **Allow-list:** one shared file, `security/allow.json`: an array of `{"tool", "id", "reason",
  "expires"}`, where `tool` is one of `trivy`, `trivy-config`, `zizmor`, `semgrep`, `zap`, `nuclei`,
  `gitleaks` and `expires` is `YYYY-MM-DD`. It started as `[]`. A `zap` id is a rule number (the
  whole rule is ignored) or `<rule number>@<url regex>` (only that rule's alerts on URLs the regex
  matches from the start are dropped, through a ZAP `OUTOFSCOPE` rules line; the rule still fails
  on every other route); prefer the scoped form. A `trivy-config` id is a check id (for example
  `DS-0002`: the check is ignored in every scanned Dockerfile) or `<check id>@<repo path>` (for
  example `DS-0002@deploy/backup/Dockerfile`: ignored in that Dockerfile only, through Trivy's own
  `paths:` in the ignore entry, so the same check still blocks on every other file); prefer the
  scoped form. `config-scan.sh` refuses a scope that is not one of the Dockerfiles it scans. A `trivy` id is a CVE id
  (ignored in every result of every image and of `deps-scan`) or `<CVE id>@<path>` (for example
  `CVE-2026-78661@usr/local/bin/cloudflared`: ignored only in the result whose Target is that path,
  through Trivy's own `paths:`; the same CVE in any other binary still blocks). For a Go binary in
  an image Trivy 0.76.1 sets the Target to the binary's path in the image, without a leading slash
  (checked by a run on the pinned Postgres and cloudflared images; `PkgPath` is empty there), so the
  scope is a path inside the image. Prefer the scoped form for a CVE in a third-party binary: a plain
  id would also hide the same Go standard library CVE in our own app binary. `image-scan.sh` and
  `deps-scan.sh` both write the ignore file through `scripts/security/trivy-ignore.sh` and refuse
  an id of another shape; `deps-scan.sh` passes a scoped id through, where it ignores nothing
  because an image path is never a manifest. The
  guard `TestSecurityAllowList` (`internal/guards`) fails on a missing field, an unknown tool, a bad
  date, an expired date, a date more than 180 days ahead, a duplicate `(tool, id)`, an id of any
  tool with whitespace, a newline or a comma (it would read as two ids: `allow.sh` prints one per
  line, and the nuclei ids are joined with commas), a `zap` id of
  another shape (its regex must compile, end in `$` and match none of a few probe URLs: the shell,
  a built file, an API route; that catches a typo or a catch-all), or a `trivy-config` id of another
  shape (the scope must be a clean repo-relative path, with no glob, to a file that exists), or a
  `trivy` id of another shape (the scope must be a clean relative path, with no glob and no `..`; it
  names a file inside an image, so it is not looked up in the repo). `scripts/security/allow.sh <tool>` prints
  that tool's live ids (expiry today or later, UTC), one per line, and refuses an id with a line
  break; each scan wrapper refuses its own ids of another shape (a space or a comma included) and
  turns them into the tool's own ignore form. gitleaks keeps its `.gitleaksignore` of fingerprints, which this
  guard does not cover. A new entry or a later expiry date needs the owner's agreement, written in
  the commit message, like `ui-probe.allow.ts` (`.claude/rules/ui.md`). The default answer to a
  finding is a code change. My call.
- CI uploads SARIF to the repo's Security tab where the tool supports it.
- **"Blocks a merge" is enforced by GitHub (owner's decision):** `main` gets branch protection (or
  a ruleset) that requires a pull request and every blocking CI job as a required status check,
  and forbids force-push and deletion. Changes reach `main` only through a green PR; no local merge
  and push to `main`. This is a GitHub setting, not a repo file: `docs/threat-model.md` "GitHub
  settings" holds the exact `gh api` command, with the tag ruleset for `v*` and the public ghcr
  packages. The ruleset only limits who can tag (the owner is on its bypass list); the check that
  the tagged commit is on `main` is `release-on-main.sh` (section 4). The owner applies the
  settings after the merge (a required check must have run first). The threat model lists them as
  `manual:` rows, "re-checked by hand after any repo settings change". The job names in CI stay
  stable, because a renamed required check silently blocks every PR.

## Where each check runs

| Check | Every push / PR | Release | Weekly CI | Local `make` |
| --- | --- | --- | --- | --- |
| Threat-model guard | yes | | | in the gate |
| Semgrep | yes | | | in the gate |
| Config scan | yes | | | in the gate |
| Image scan | yes | before push | released images | `make image-scan` |
| SBOM + signing | | yes | | |
| ZAP + Nuclei (CI app) | yes | | | `make dast` |
| Fuzz seed corpus | yes (in `make test`) | | | in the gate |
| Fuzzing (`make fuzz`) | | | `make fuzz FUZZTIME=30s` | `make fuzz` (not in the gate) |
| Live scan | never | never | never | `make scan-live URL=…` |

My call: the tools that run offline-ish in Docker join the local gate (`CLAUDE.md` "Gate"), so an
agent sees the same red locally. The scans that need the built app (ZAP, Nuclei) stay CI plus a
separate `make dast`, like `make e2e`.

## Later

- Falco and Tetragon (need the host).
- Cross-user ownership tests, when staff accounts arrive.

## Left to the builder

- Exact tool versions, images and flags (verified against each tool's docs; pinned per "Pins").
- Checkov vs Trivy config, and the Actions-specific scanner (decided: section 3).
- The phase split and order, with the image scan and the running-app scan (with the headers fix)
  first, because they close the two gaps found today.
