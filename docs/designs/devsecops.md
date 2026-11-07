# open-customer — Security pipeline

Status: approved by the owner (2026-11-06). Being built: the threat model, the allow-list, the image
scan, the secret scan, the running-app scan and the config scan (with the compose hardening and the
supply-chain pins) exist; the code scan, the fuzz tests, SBOM and signing, the live scan and the
manual audit do not yet.

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
- `CLAUDE.md` says `docs/designs/release-1.md` is the only architecture source. The change that
  builds this pipeline updates that line to name this document too.
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
  or `Fuzz` function some `_test.go` declares), `file:<repo path>`, `make:<target>`,
  `ci:<workflow>/<job>`, `accepted: <reason>`, `out of scope: <who owns it>`, `manual: <what and how
  often>` (GitHub settings), and `gap: Task <N>` for a threat a later task of the build plan closes
  (allowed only while the pipeline is being built; its last task removes the word from the guard).
  The allow-list is not a check, so it is never a reference.
- The guard `TestThreatModel` (`internal/guards`, run by `make test`) fails on a row with an empty
  "Covered by", a reference of another type, a test no `_test.go` declares, a file that does not
  exist, a make target the Makefile lacks, a CI job the workflow lacks, or a duplicate ID.
- The one-time manual audit (section 8) adds rows.

### 2. Code scan (Semgrep)

- Semgrep's Go and TypeScript security rules, plus repo-specific rules where a security obligation
  is now prose only (for example: no `slog` call logs a request body; every non-GET handler runs in
  `db.WithTx`). Each custom rule is seen red once.
- gosec stays in golangci-lint. Where both find the same thing, keep the existing one.
- Rules are pinned, never fetched unpinned at run time: the Semgrep version is pinned per "Pins",
  and registry rules are used only at a pinned version or copied in when their license allows it
  in a public repo (the builder checks the current license). Custom rules live in the repo. My call.

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
  seen failing without), `USER 65532:65532` in the app image, `persist-credentials: false` on every
  checkout, no Go cache in the release workflow, and `packages: write` on the release job only.
- Supply-chain pins: every `uses:` is a full commit SHA with its `# vX.Y.Z` and every image a
  `<tag>@sha256:<index digest>` (Dockerfiles, compose, CI services, the Makefile, the deploy
  scripts), checked by the guard `TestSupplyChainPins`; `make check-pins` keeps the copies of the
  Postgres pin, digest included, equal. Dependabot bumps the SHA or digest and the version together
  where it reads the file (not the Makefile).

### 4. Image scan and supply chain (Trivy)

- On every push: build the app and backup images and scan them, plus the pinned `postgres` and
  `cloudflared` images from `deploy/compose.yaml`.
- In the release workflow: scan before push; a blocking finding stops the release.
- Weekly scheduled CI run: scan the images of the latest release tag only (deployers upgrade to the
  latest). A finding that meets the blocking policy fails the run, and GitHub emails the owner about
  a failed scheduled run with no setup. Until the first `v*` tag exists, the job writes "NOT RUN: no
  release tag" in its summary and passes. My call.
- My call: the release also writes an SBOM and signs the images keylessly with cosign through
  GitHub OIDC. Keyless signing stores no key, so it keeps anti-goal 1.
- My call: `deploy/scripts/upgrade.sh` verifies the signature before it pulls, running cosign from
  its pinned image (no install on the deployer's machine). A bad or missing signature stops the
  upgrade with nothing changed; `make test-deploy` gets a test for it.

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
  http only on localhost).
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
  `TestSecurityHeadersOnEveryResponse`.

### 7. Fuzz tests

- Go native fuzz tests on the parsers that take outside input: JSON body decoding, the multipart
  upload handlers, template parsing, and the backup status file reader.
- The gate runs each seed corpus as a normal test; a longer fuzz run is a manual or scheduled job.
  A crash found by fuzzing becomes a seed and a fix.

### 8. Manual audit (once, then on big changes)

- A one-time security review of the code for logic bugs no scanner finds (the `security-audit`
  skill fits). Findings become threat-model rows and fixes.

### 9. Live scan (`make scan-live URL=…`)

- Runs on the deployer's own machine against any deployment, behind any proxy. Uses no key.
- Nuclei (safe, non-intrusive templates only), ZAP baseline (passive), and TLS and header checks.
- Exits non-zero on a blocking finding and writes a report file. `deploy/README.md` documents the
  exit code, the report and how to schedule it with cron.
- Never runs in GitHub.
- Its allow-list is a local file the deployer owns, kept out of git (`deploy/.scan-live.allow`,
  git-ignored), because what a proxy adds or strips differs per deployment. My call.

## Blocking policy

- **CVE scanners** (Trivy on images and dependencies): high or critical with a fixed version
  available blocks. Medium and low are reported, not blocking.
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
  on every other route); prefer the scoped form. The guard `TestSecurityAllowList`
  (`internal/guards`) fails on a missing field, an unknown tool, a bad date, an expired date, a date
  more than 180 days ahead, a duplicate `(tool, id)`, or a `zap` id of another shape (its regex must
  compile, end in `$` and match none of a few probe URLs: the shell, a built file, an API route;
  that catches a typo or a catch-all). `scripts/security/allow.sh <tool>` prints
  that tool's live ids (expiry today or later, UTC), one per line, and each scan wrapper turns them
  into the tool's own ignore form. gitleaks keeps its `.gitleaksignore` of fingerprints, which this
  guard does not cover. A new entry or a later expiry date needs the owner's agreement, written in
  the commit message, like `ui-probe.allow.ts` (`.claude/rules/ui.md`). The default answer to a
  finding is a code change. My call.
- CI uploads SARIF to the repo's Security tab where the tool supports it.
- **"Blocks a merge" is enforced by GitHub (owner's decision):** `main` gets branch protection (or
  a ruleset) that requires a pull request and every blocking CI job as a required status check,
  and forbids force-push and deletion. Changes reach `main` only through a green PR; no local merge
  and push to `main`. This is a GitHub setting, not a repo file: the plan writes the exact setting
  (and the `gh api` command that applies it), and the owner applies it or approves it being
  applied. The threat model lists it as a row whose check is "GitHub setting, re-checked by hand".
  The job names in CI stay stable, because a renamed required check silently blocks every PR.

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
