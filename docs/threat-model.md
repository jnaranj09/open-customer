# open-customer — Threat model

Every way the app can be attacked or lose data, and the check that covers each one. The security
pipeline (`docs/designs/devsecops.md`) serves this list: a new entry point, a new scan or a new
security fix adds or changes a row in the same change.

The guard `TestThreatModel` (`internal/guards/threat_model_test.go`, run by `make test`) reads the
table under "Threats" and fails on a row with no "Covered by", or on a reference that does not
resolve.

## How to read "Covered by"

Each cell holds one or more references, separated by `; `:

| Reference | Meaning | The guard checks |
| --- | --- | --- |
| `test:<Name>` | A Go `Test` or `Fuzz` function | some `_test.go` declares it |
| `file:<path>` | A file that carries the check (a shell test, a web test, a config) | the path exists |
| `make:<target>` | A make target that runs the check | the Makefile has the target |
| `ci:<workflow>/<job>` | A CI job that runs the check | the job is in `.github/workflows/<workflow>` |
| `accepted: <reason>` | A risk the owner accepted (`docs/designs/release-1.md` "Accepted risks") | a reason is given |
| `out of scope: <who owns it>` | Outside what the repo can check | the owner is named |
| `manual: <what and how often>` | Checked by hand (GitHub settings) | the text is given |

A `test:` row names the test that fails when the defence breaks, not every test nearby. There is no
`gap:` type: a threat with no check is `accepted:`, `out of scope:` or `manual:`, and the guard
refuses a `gap:` reference.

## Scope

In scope: this repo, its CI and release workflows, the two images it builds (app and backup), the
pinned third-party images in `deploy/compose.yaml` (Postgres, cloudflared), and the running app
behind any front proxy.

Out of scope (rows T80 and T81, so the list is visibly complete): the host machine (OS, disk,
Docker daemon, `deploy/.env` at rest, `docker logs`), and the front proxy or tunnel provider's own
dashboard and settings (Cloudflare or any other).

## Threats

| ID | Asset | Threat | Entry point | Covered by |
| --- | --- | --- | --- | --- |
| T01 | client data | A route answers personal data without a session | JSON API | test:TestEveryOperationNeedsASessionUnlessPublic; test:TestRequireSessionOnAPatternOutsideThePublicSetIs401 |
| T02 | client data | A new route is made public by mistake in the spec | api/openapi.yaml | test:TestPublicOperationsAreExactlyTheSetupLoginAndPushKeyOnes; test:TestPublicOperationsRejectsSecurityStep7CannotCheck |
| T03 | the session | A forged or garbage session cookie is accepted | session cookie | test:TestMeWithGarbageCookieIs401; test:TestAuthenticateBadToken |
| T04 | the session | An expired session still works | session cookie | test:TestAuthenticateExpired; test:TestMeWithExpiredSessionIs401 |
| T05 | the session | A database leak gives working session or setup tokens | sessions and setup_tokens tables | test:TestTokenStoredAsHash; test:TestSetupTokenStoredAsHash |
| T06 | the session | The cookie is readable by script, sent cross-site or sent over plain http | session cookie | test:TestSessionCookieAttributes; test:TestPostSetupCookieIsSecureOnHTTPS |
| T07 | the session | Logout leaves the session usable | POST /auth/logout | test:TestLogoutRevokes; test:TestLogoutExpiresCookieAndRevokes |
| T08 | the session | A lost phone keeps its session after a password change or reset | POST /auth/password, reset-password | test:TestChangePasswordRevokesOtherSessions; test:TestResetPasswordSetsMustChangeAndRevokesAll |
| T09 | client data | Cross-site request forgery: another site makes her browser write | every non-GET request | test:TestOriginCheck; test:TestPostSetupNoOriginIs403; test:TestLogoutWithoutOriginIs403 |
| T10 | client data | A temporary password is used for anything but changing it | must-change-password gate | test:TestMustChangePasswordGate; test:TestAllowedWhileMustChangeSetMatchesSpec; test:TestPatchSettingsMustChangeIs403 |
| T11 | the account | Password guessing from one address | POST /auth/login | test:TestLoginSixthAttemptIs429; test:TestIPLimitFiveThenBlocked; test:TestIPv6SameSlash64SharesBucket |
| T12 | the account | Password guessing spread over many addresses | POST /auth/login | test:TestUsernameDelayDoubles; test:TestUsernameDelayCappedAtThirtySeconds; test:TestLoginWrongPasswordDelaysTheNextAttempt |
| T13 | the account | A spoofed CF-Connecting-IP header escapes the rate limit | client IP | test:TestLoginRateLimitTrustedVsUntrustedPeer; test:TestClientIP; test:TestUnparsablePeerIsNotTrusted |
| T14 | the account | Login timing tells which usernames exist | POST /auth/login | test:TestLoginUnknownUserVerifiesAgainstTheDummyHash; test:TestDummyHashUsesDefaultParams |
| T15 | the account | A stolen database gives readable passwords | users table | test:TestHashFormatIsPHC; test:TestHashUsesAFreshSalt; test:TestVerifyHonoursStoredParams |
| T16 | the account | Guessing the current password through the change-password form | POST /auth/password | test:TestChangePasswordWrongCurrentDelaysTheNextTry; test:TestChangePasswordWrongCurrentDelaysTheUsername |
| T90 | the account | A weak or short password is accepted (setup, change, reset) | POST /setup, POST /auth/password, reset-password | test:TestSetupShortPassword; test:TestPasswordTooShort; test:TestChangePasswordTooShort; test:TestTempPasswordAlphabetAndLength |
| T17 | the account | Password-only login on the public internet | POST /auth/login | accepted: no second factor, mitigated only by Argon2id, the login rate limit and the per-username delay (release-1 Accepted risks) |
| T18 | the account | Anyone who knows her username can slow her logins | POST /auth/login | accepted: the per-username delay applies to everyone, an attacker can keep her waiting but cannot get in, and her open sessions are not affected (release-1 Accepted risks) |
| T19 | the whole app | Someone else completes setup before the owner | POST /setup | test:TestSetupTokenGarbage; test:TestPostSetupBadTokenIs410; test:TestSetupTokenExpired; accepted: POST /setup has no rate limit, because the setup token is 256 random bits, lives 24 h and is single-use, so guessing it is not practical, and the design asks for no limit there (security audit A7) |
| T20 | the whole app | A setup link is used twice, or setup runs again after the owner exists | POST /setup | test:TestSetupTokenCannotBeReused; test:TestPostSetupTwiceIsSetupClosed; test:TestSetupRefusedOnceUserExists; test:TestConcurrentSetupWithOneTokenCreatesOneUser |
| T21 | the whole app | The setup token leaks through a request log or a proxy (it travels in the URL fragment) | setup link | test:TestServePrintsSetupLinkOnEmptyDB; test:TestServeNoSetupLinkWhenUserExists; test:TestSetupLinkWhenUserExists |
| T22 | client data | A password, token or request body is written to the logs | logs | test:TestLoginAttemptNeverShowsThePassword; test:TestDecodeJSONLogNeverQuotesTheDecodeError; test:TestLoadNeverQuotesThePassword; test:TestForbiddenOriginIsProblemAndLogsCodeNotValue; file:security/semgrep/no-slog-of-request-body.yaml; make:sast |
| T23 | client data | A database or file error shows internals (SQL text, file paths) to the client or the log | error responses | test:TestInternalHidesTheError; test:TestPostSetupDBErrorLogsNoRawMessage; test:TestDBErrorInsideAJoinLogsNoPostgresText; test:TestGetMediaFileMissingIs500WithoutPath; test:TestIconReadErrorLogsNoPath; file:security/semgrep/no-raw-error-to-client.yaml; make:sast |
| T24 | the whole app | One bad request crashes the server | every handler | test:TestPanicBecomesInternalProblem; test:TestRecoverPanicLetsAbortHandlerThrough |
| T25 | client data | A malformed, oversized or unexpected JSON body is half accepted | JSON request body | test:TestDecodeJSON; test:TestPostSetupUnknownFieldIs422; test:TestPostSetupMalformedIs422; test:TestLoginOversizeIs413; test:FuzzDecodeJSON; make:fuzz; ci:security-weekly.yml/fuzz |
| T26 | client data | A bad path or query parameter reaches a module | path and query params | test:TestErrorHandlerFuncWritesValidation; test:TestPatchAttributeBadUUIDIs422; test:TestListAppointmentsBadDateIs422 |
| T27 | client data | SQL injection | every database query | make:lint; file:security/semgrep/no-dynamic-sql.yaml; make:sast |
| T28 | client data | Stored script (XSS): a client name or note runs as code in the app | the web app | file:web/eslint.config.js; make:lint; file:security/semgrep/ts-no-inner-html.yaml; file:security/semgrep/svelte-no-html-tag.yaml; make:sast; file:web/vite.config.ts; make:dast; ci:ci.yml/security |
| T29 | the session | A script injected by any means reads data or acts as her | the web app | test:TestSessionCookieAttributes; file:web/vite.config.ts; test:TestSecurityHeadersOnEveryResponse; file:security/semgrep/ts-no-eval.yaml; make:sast; make:dast |
| T30 | the session | Clickjacking: the app is framed by another site | every response | test:TestSecurityHeadersOnEveryResponse; make:dast; ci:ci.yml/security |
| T31 | client data | MIME sniffing turns an uploaded file into a script | photos, icons | test:TestUploadMediaOK; test:TestUploadIconOK; test:TestSecurityHeadersOnEveryResponse; test:TestSetIconsRejectsNonPNG; test:TestUploadMediaNotJPEGIs415 |
| T32 | client data | Another site reads API responses with her cookie (CORS) | JSON API | test:TestSecurityHeadersOnEveryResponse; make:dast; ci:ci.yml/security |
| T33 | client data | Known weaknesses a scanner finds in the running app (headers, misconfiguration, exposed files) | the running app | make:dast; ci:ci.yml/security; file:scripts/security/tests/run.sh |
| T34 | client data | The deployed app, behind its real proxy, has a weakness the CI app does not | the deployment | make:scan-live; file:scripts/security/scan-live.sh; test:TestSecurityHeadersOnEveryResponse; file:scripts/security/tests/run.sh; manual: the deployer runs `make scan-live URL=https://…` against the real deployment, after each upgrade and on a schedule (deploy/README.md section 14) |
| T35 | photos | A photo upload that is not a JPEG, or is too large, is stored | POST /media | test:TestUploadMediaNotJPEGIs415; test:TestUploadMediaFullOver10MBIs413; test:TestUploadMediaRequestOver12MBIs413; test:TestUploadMediaNotMultipartIs415; test:TestSaveRejectsUnreadableJPEG; test:FuzzMultipartUpload; make:fuzz; ci:security-weekly.yml/fuzz |
| T36 | photos | A half-failed upload leaves files without a row, or a row without its file | POST /media | test:TestSaveWritesFilesBeforeRow; test:TestSaveInsertFailureRemovesFiles; test:TestUploadMediaCommitFailureRemovesFiles |
| T37 | photos | A photo is filed under the wrong client | POST /media | test:TestUploadMediaMismatchIs422; test:TestSaveMismatchedClientIs422; test:TestSaveArchivedOrUnknownClientIs422 |
| T38 | the whole app | An icon upload that is not a PNG, or is too large, is stored | POST /settings/icon | test:TestUploadIconNotPNGIs415; test:TestSetIconsRejectsNonPNG; test:TestUploadIconPartOver1MBIs413; test:TestUploadIconRequestOver3MBIs413; test:TestUploadIconMalformedMultipartIs422; test:FuzzMultipartUpload; make:fuzz; ci:security-weekly.yml/fuzz |
| T39 | photos | A path in a request reads a file outside the build or media folder | static files, GET /media/{id} | test:TestGetMediaBadUUIDIs422; test:TestDirectoryHasNoListing; test:TestDotDotPathRedirectsToTheCleanPath; test:TestEncodedDotDotPathServesTheAppShell; test:TestGetMediaKeyOutsideTheMediaFolderIsRefused |
| T40 | client data | The export is downloaded without a session | GET /export | test:TestExportNeedsSession |
| T41 | client data | A failed export looks like a complete file | GET /export | test:TestExportAbortsAfterStreamStarts; test:TestExportFailsBeforeStreamIsProblem |
| T42 | client data | A client name or note starting with = or + runs as a formula when the CSV is opened in a spreadsheet | GET /export | test:TestExportNeutralizesFormulaCells |
| T43 | the whole app | A push subscription with a bad endpoint or keys is stored | POST /push/subscriptions | test:TestSubscribePushHttpEndpointIs422; test:TestSubscribePushPrivateEndpointIs422; test:TestSubscribeRejects; test:TestSubscribeRefusesAnUncheckedSubscription; test:TestSubscribePushMissingKeysIs422; test:TestSubscribePushNeedsSession |
| T44 | the home network | A push endpoint pointing at a private address makes the server send requests into the home network | POST /push/subscriptions, the daily push | test:TestSubscribeRejects; test:TestIsPublic; test:TestSubscribePushPrivateEndpointIs422; test:TestSendAllNeverDialsANonPublicAddress; test:TestSendAllFollowsNoRedirect |
| T45 | the whole app | The VAPID key pair is lost, so every push subscription stops working | push keys | test:TestEnsureKeysCreatesOnce; make:test-deploy |
| T46 | the whole app | An expired or deleted push subscription fails the daily push | Web Push service responses | test:TestSendAllGoneDeletesSubscription; test:TestSendAllServerErrorKeepsSubscription; test:TestSendAllNetworkErrorCountsNotDelivered |
| T47 | the whole app | A dead push subscription shows only on the status screen | daily push | accepted: the re-subscribe check fixes most cases on the next open, otherwise she notices the missing morning notification (release-1 Accepted risks) |
| T48 | logs | Client error reports flood the log or carry oversized text | POST /client-errors | test:TestLogClientErrorRejectsBadFields; test:TestPostClientErrorEleventhIs429; test:TestLogClientErrorCutsLongStack; test:TestPostClientErrorNeedsSession |
| T49 | logs | A client error report writes raw text from the app into the log | POST /client-errors | accepted: the message and stack come from our own PWA and are the only way to debug it, with length limits and a rate limit (release-1 Observability) |
| T50 | client data | A template or a WhatsApp link breaks out of its text, or a body of many bad tokens makes a huge error response | templates, reminder links | test:TestParseRejects; test:TestParseManyBadBracesIsLinearAndOrdered; test:TestRenderValueAndFallback; test:TestWhatsAppLinkEncoding; test:FuzzParseTemplate; make:fuzz; ci:security-weekly.yml/fuzz |
| T51 | client data | A stored attribute value of the wrong shape breaks a screen or is accepted on write | attribute values | test:TestValidateRejectedWrites; test:TestFormatBadStoredValueLogsCode |
| T52 | client data | Cross-user access to another owner's records | every route | accepted: release 1 has one user, so cross-user ownership tests do not apply yet (devsecops design section 5) |
| T53 | client data | Two of her devices save at the same moment and double-book a slot | POST /appointments | accepted: the overlap check takes no lock, one owner is effectively one writer, and she sees both appointments and can move one (release-1 Accepted risks) |
| T54 | client data | Public routes show business data | /healthz, /readyz, manifest, icons, app shell | test:TestPublicRoutesNeedNoSession; test:TestManifestRouteNoSessionNoCache; accepted: nothing is public except setup, the health checks, the manifest, the icons and the app shell (release-1 Security summary) |
| T55 | the whole app | A client that never finishes its request headers or its body, or holds an idle connection, keeps connections open | the HTTP server | test:TestHTTPServerTimeouts; make:lint |
| T56 | the whole app | A flood of requests takes the home PC down | the public URL | out of scope: the front proxy or tunnel provider the deployer chose |
| T57 | client data | Cached data stays readable on a phone after its session is closed | the phone cache | file:web/src/lib/logout.test.ts; accepted: the cache is cleared on the phone's next request, and a lost phone is protected by its own screen lock until then (release-1 Accepted risks) |
| T58 | client data | A bad row in the phone cache or a bad API response is shown as data | the phone cache, API responses | file:web/src/lib/cache/store.test.ts; file:web/src/lib/api/client.test.ts |
| T59 | the whole app | Bad .env values start the server in an unsafe state | .env | test:TestLoadRejectsBadValues; test:TestLoadListsEveryMissingVariable; test:TestLoadAcceptsLocalHTTP |
| T60 | the backups | A backup is written to the PC's own disk when the backup disk is missing | backup.sh | file:deploy/tests/backup_test.sh; make:test-deploy; test:TestBackupMissingFileIsRed; test:TestBackupRedRules |
| T61 | the backups | A backup of new, empty volumes replaces the real backups | backup.sh | file:deploy/tests/backup_test.sh; make:test-deploy |
| T62 | the backups | A malformed or stale backup status file hides a failed backup | backup status file | test:TestReadBadContentIsInvalid; test:TestReadMissingFileIsErrMissing; test:TestBackupRedRules; test:FuzzBackupStatusRead; make:fuzz; ci:security-weekly.yml/fuzz |
| T63 | the backups | Backups are readable by whoever gets the backup disk, a published placeholder password encrypts them, or a plaintext dump stays beside them | the backup target, .env, the backup-status volume | file:deploy/backup/backup.sh; file:deploy/tests/backup_test.sh; make:test-deploy; test:TestLoadRejectsBadValues; test:TestPlaceholderSecretsMatchEnvExample; test:TestPlaceholderGapsProbes |
| T64 | the backups | Fire or theft takes the PC and its backup disk together | the backup target | accepted: backups stay in the same house, off-site backup is a future improvement (release-1 Accepted risks) |
| T65 | client data | An upgrade breaks the schema with no way back | upgrade.sh, serve | test:TestMigrateBlocksWithOldCopy; test:TestMigrateBlocksOnMissingStatusFile; test:TestAdminCommandsRefusePendingMigrations; file:deploy/tests/scripts_test.sh; make:test-deploy |
| T66 | client data | A restore runs without confirmation or leaves the app on the wrong version | restore.sh | file:deploy/tests/scripts_test.sh; make:test-deploy |
| T67 | deploy/.env | A killed upgrade or restore leaves a copy of .env (every secret) behind | upgrade.sh, restore.sh | file:deploy/tests/scripts_test.sh; make:test-deploy |
| T68 | the images | An upgrade, a restore, a manual backup or the first install pulls an image that was not built by this repo's release workflow, or one the tag was moved to after its signature was checked | upgrade.sh, restore.sh, backup-now.sh, pull.sh, deploy/compose.yaml | file:deploy/scripts/lib.sh; file:deploy/scripts/pull.sh; file:deploy/tests/scripts_test.sh; make:test-deploy; make:check-pins; test:TestComposeHardening; out of scope: the deployer's own hand-typed `docker compose` commands pull with no signature check, and deploy/README section 6 says run scripts/pull.sh first |
| T69 | the images | A known high or critical CVE ships in the app or backup image | Dockerfile, deploy/backup/Dockerfile | make:image-scan; ci:ci.yml/security; ci:release.yml/images; file:scripts/security/tests/run.sh; make:test-sec |
| T70 | the images | A known CVE in the pinned Postgres or cloudflared image | deploy/compose.yaml | make:image-scan; ci:release.yml/images; ci:security-weekly.yml/rescan; ci:ci.yml/security |
| T71 | the images | A CVE found after an image shipped goes unnoticed | released images | ci:security-weekly.yml/rescan; make:image-scan |
| T72 | the images | The app runs as root or with a shell an attacker could use | Dockerfile | file:Dockerfile; make:config-scan; ci:ci.yml/gate; test:TestComposeHardening |
| T73 | the home network | The tunnel reaches the home network, so a bug in the app exposes the Compose network | deploy/compose.yaml | test:TestComposeHardening; accepted: the app runs as non-root in a distroless image, every container drops all capabilities and adds back only what it needs (backup DAC_OVERRIDE, restore CHOWN, DAC_OVERRIDE and FOWNER) and none publishes a port, but the Compose network itself is reachable from the app (release-1 Accepted risks) |
| T74 | the build | A dependency with a known vulnerability (Go) | go.mod | make:vuln; ci:ci.yml/gate; file:.github/dependabot.yml |
| T75 | the build | A dependency with a known vulnerability (npm) | web/package.json | ci:ci.yml/gate; file:.github/dependabot.yml |
| T91 | the build | A base, service or tool image, or a package an image installs, is swapped under its tag or version (no CVE) | Dockerfile, deploy/backup/Dockerfile, deploy/compose.yaml, security/tools.Dockerfile, .github/workflows | test:TestSupplyChainPins; file:security/tools.Dockerfile; file:.github/dependabot.yml; make:check-pins; file:scripts/security/tests/run.sh; ci:ci.yml/gate; accepted: the backup image installs bash, jq and restic from the Alpine mirror unpinned, because the base image pins the Alpine release by digest, apk verifies each package with that image's Alpine signing keys, and the weekly rescan blocks a new CVE, and restic stays the Alpine package because it passes make image-scan while both official restic images (0.18.1 and 0.19.1, checked 2026-11-07) fail it (controller ruling, Task 9). The app image installs pnpm@12.16.1 with npm, which never lets a published version be replaced. TestSupplyChainPins refuses any other apk package, and any package without a reason |
| T95 | the home network | The backup or restore container, which runs as root with a shell, is used to reach the host or the other volumes | deploy/backup/Dockerfile, deploy/compose.yaml | test:TestComposeHardening; make:config-scan; ci:ci.yml/gate; accepted: the backup image runs as root because backup and restore write a host folder of any owner and restore chowns the media to 65532, mitigated by cap_drop ALL plus only DAC_OVERRIDE (restore adds CHOWN and FOWNER), no-new-privileges and no ports, and Trivy DS-0002 is allowed on deploy/backup/Dockerfile only (owner decision, 2026-11-06, release-1 Accepted risks) |
| T96 | client data | A failed commit is shown as saved, or a write skips its transaction | every non-GET handler | test:TestCommitFailureIsNotShownAsSaved; file:security/semgrep/non-get-handler-in-withtx.yaml; test:TestSemgrepWriteVerbsMatchTheSpec; make:sast |
| T92 | the build | A Go or npm dependency is swapped for a different version or content | go.sum, web/pnpm-lock.yaml | file:go.sum; file:web/pnpm-lock.yaml; file:Dockerfile; ci:ci.yml/gate |
| T93 | the build | A package already in the lockfile is malicious with no CVE | go.sum, web/pnpm-lock.yaml | accepted: no tool finds a malicious package that has no CVE, so the controls are the lockfiles and review of each Dependabot bump |
| T76 | the build | A GitHub Action is swapped under its tag, or a workflow has too-wide permissions or script injection | .github/workflows | test:TestSupplyChainPins; make:config-scan; file:scripts/security/tests/run.sh; ci:ci.yml/gate |
| T77 | the build | A secret is committed to the repo | git history | make:secrets; ci:ci.yml/gate |
| T94 | the build | A token for an outside design tool is committed in the design file | design/soft-glass.pen | accepted: Pencil's fileToken is likely a document id, and even if it is a token it reaches only this design, which is public in this repo and holds no PII, and the value is already in public history, which is never rewritten (owner decision, 2026-11-06); file:.gitleaksignore |
| T78 | the build | A release ships images nobody can verify, with no list of what is in them, or a v* tag on a commit that is not on main gives a release that verifies | release workflow | ci:release.yml/images; file:scripts/security/release-sign.sh; file:scripts/security/release-on-main.sh; file:scripts/security/tests/run.sh; make:test-sec; manual: the first v* tag's release run proves the keyless signature and the SBOM attestation (no OIDC token outside Actions), then once per release when upgrade.sh verifies it; manual: ghcr packages open-customer and open-customer-backup set to public after the first release, so a deployer pulls and verifies without logging in |
| T79 | the build | A change reaches main without a green CI run, or a v* tag is pushed on a commit that is not on main | GitHub settings, release workflow | ci:release.yml/images; file:scripts/security/release-on-main.sh; file:scripts/security/tests/run.sh; make:test-sec; manual: branch protection on main, re-checked by hand after any repo settings change; manual: tag ruleset limiting who creates, moves or deletes v* tags, defence in depth only because the owner is on its bypass list, set once in GitHub settings and re-checked after any settings change |
| T80 | the host | The OS, the disk, the Docker daemon, deploy/.env at rest and docker logs (which hold the setup link) | the host machine | out of scope: the deployer who owns the host |
| T81 | the proxy | The front proxy or tunnel settings (cache rules, HSTS, bot rules) | the proxy dashboard | out of scope: the deployer, in their proxy or tunnel provider's dashboard |
| T82 | client data | Cloudflare sees decrypted traffic at its edge, because TLS ends there | the tunnel | accepted: accepted for the reference install and written in deploy/README.md (release-1 Accepted risks) |
| T83 | client data | Database traffic is not encrypted | the Compose network | out of scope: the deployer who owns the host, the traffic never leaves the Compose network |
| T84 | client data | One person and one PC: nobody else can upgrade, restore or reset her password | operations | accepted: her data survives in the backups, and the handover checklist gives her a sealed copy of the restic password (release-1 Accepted risks) |
| T85 | client data | The PC is off: no push, no writes, no backups while it is off | operations | accepted: she agreed to this (release-1 Accepted risks) |
| T86 | client data | Ley 8968 (Costa Rica personal data law) is not checked yet | operations | accepted: the author checks her obligations before her real data goes in, a step in the pre-handover checklist in deploy/README.md (release-1 Accepted risks) |
| T87 | client data | Logic bugs no scanner finds | the code | file:.claude/agents/code-reviewer.md; manual: a whole-repo security audit against this list, first done on 2026-11-07 (security pipeline Task 9, findings A1 to A10), repeated when a change adds an entry point or before a release that does |
| T88 | photos | The front proxy's upload size or response timeout cuts a photo upload or the export | the public URL | file:web/src/lib/photos.test.ts; test:TestExportOK; test:TestExportAbortsAfterStreamStarts |
| T89 | client data | iOS clears the PWA's storage and the phone shows nothing | the phone cache | file:web/src/lib/cache/store.test.ts; file:web/src/lib/cache/refresh.test.ts |

## GitHub settings

These settings live in GitHub, not in a repo file. The owner applies them, once, after the branch is
merged (a required check must have run on the repo first). The rows T78 and T79 point here. They are
re-checked by hand after any repo settings change.

### Branch protection on `main`

Required checks are the CI job names `gate` and `security`; a renamed job silently blocks every PR.
No required review: the owner cannot approve her own PR. The repo merges with merge commits, so
linear history is off. Free plan, public repo.

```sh
gh api -X PUT repos/jnaranj09/open-customer/branches/main/protection --input - <<'JSON'
{
  "required_status_checks": {"strict": true, "checks": [{"context": "gate"}, {"context": "security"}]},
  "enforce_admins": true,
  "required_pull_request_reviews": {"required_approving_review_count": 0, "dismiss_stale_reviews": true},
  "restrictions": null,
  "required_linear_history": false,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
JSON
```

Confirm: `gh api repos/jnaranj09/open-customer/branches/main/protection`.

### Tag ruleset for `v*`

Two controls guard a `v*` tag, and they do different things:

- The release workflow enforces the commit. Its first step, `scripts/security/release-on-main.sh`,
  fails the release unless the tagged commit is an ancestor of `main`, before anything is built,
  scanned or pushed. It fails closed (a missing SHA, a shallow clone and a failed fetch are red).
  Its limit: an ancestor of `main` has not necessarily passed CI.
- The ruleset below limits who can create, move or delete a `v*` tag. The owner is on its bypass
  list, so it does not stop her; it is defence in depth against anyone else with push access.

The body follows GitHub's documented create-ruleset shape (`POST repos/{owner}/{repo}/rulesets`,
`target: tag`, rule types `creation`, `deletion` and `non_fast_forward`). The docs list no
`update` parameter for tags (`update_allows_fetch_and_merge` is described for branches), so the
`update` rule is left out. Not confirmed against the docs: that `non_fast_forward` applies to tags,
and the Admin role `actor_id` 5 (from memory). If the call is refused, use the settings page
instead (Settings, Rules, Rulesets, New tag ruleset, target `v*`, restrict creations and
deletions, block force pushes, bypass list: Repository admin role).

```sh
gh api -X POST repos/jnaranj09/open-customer/rulesets --input - <<'JSON'
{
  "name": "release-tags",
  "target": "tag",
  "enforcement": "active",
  "bypass_actors": [{"actor_type": "RepositoryRole", "actor_id": 5, "bypass_mode": "always"}],
  "conditions": {"ref_name": {"include": ["refs/tags/v*"], "exclude": []}},
  "rules": [{"type": "creation"}, {"type": "deletion"}, {"type": "non_fast_forward"}]
}
JSON
```

Not run against the repo yet. Confirm with `gh api repos/jnaranj09/open-customer/rulesets`.

### ghcr package visibility

After the first release pushes the images, each package starts private. Set both to public so a
deployer pulls and verifies without logging in: github.com, the repo's Packages, `open-customer`,
Package settings, Danger Zone, Change visibility, Public; then the same for `open-customer-backup`.
Check with `docker logout ghcr.io` and a pull of the released tag.

### Re-check

After any repo settings change, read the three back: the branch protection and rulesets commands
above, and the packages' visibility on their pages.
