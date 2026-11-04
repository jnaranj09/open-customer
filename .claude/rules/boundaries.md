---
paths:
  - 'cmd/**'
  - 'internal/httpapi/**'
  - 'internal/platform/config/**'
  - 'internal/platform/backupstatus/**'
  - 'internal/status/**'
  - 'internal/media/**'
  - 'internal/push/**'
  - 'internal/attributes/**'
  - 'internal/db/migrate.go'
  - 'web/src/lib/api/**'
  - 'web/src/lib/cache/**'
  - 'web/src/service-worker.ts'
  - 'web/src/routes/**'
  - 'deploy/**/*.sh'
---

# Rule: fail-closed boundaries

Every outside input is parsed at exactly ONE boundary, into a precise type, and fails closed.

```
// Go — the one decode site for JSON request bodies (helper names illustrative; the
// problem package defines the real ones).
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // our own client: an unknown field is a bug, not an extension
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			slog.WarnContext(r.Context(), "body rejected", "code", "payload_too_large", "path", r.URL.Path)
			return problem.PayloadTooLarge()
		}
		// Never log err: a decode error can quote the body.
		slog.WarnContext(r.Context(), "body rejected", "code", "body_invalid", "path", r.URL.Path)
		return problem.Validation()
	}
	return nil
}

// TypeScript — the one parse site for an API response (web client).
const parsed = DueListSchema.safeParse(body);
if (!parsed.success) {
  logClientError({ code: 'response_invalid', path: '/reminders/due' }); // never the body
  return { kind: 'error', code: 'response_invalid' };
}
return { kind: 'ok', value: parsed.data };
```

| Boundary | Parse site | On bad input | Bad-input test |
| --- | --- | --- | --- |
| JSON request body | `decodeJSON` in the httpapi server (typed struct, `DisallowUnknownFields`; the module validates each field); `decodePatch` for PATCH bodies (also refuses `null` in a field that cannot be cleared) | 422 `validation` or 413 `payload-too-large` problem JSON | httpapi tests: unknown field, malformed JSON, over-limit body (`TestDecodeJSON` in P1; P2 per route, item 37: `TestPostSetupUnknownFieldIs422`, `TestPostSetupMalformedIs422`, `TestPostSetupOversizeIs413`, `TestLoginBadInputIs422`, `TestLoginOversizeIs413`, `TestChangePasswordBadInput`, `TestPatchSettingsCurrencyIs422`, `TestPatchSettingsBadInput`) |
| Path and query params | oapi-codegen binding, `ErrorHandlerFunc` | 422 `validation` problem JSON | `ErrorHandlerFunc` unit test (P1); per-route bad UUID and bad date (P3, item 37; first: `TestPatchAttributeBadUUIDIs422`; appointments: `TestGetAppointmentBadUUIDIs422`, `TestListAppointmentsBadDateIs422`, `TestListAppointmentsMissingToIs422`) |
| `Origin` header, session cookie | httpapi middleware | 403 / 401 problem JSON | middleware tests: wrong Origin (P1), bad cookie `TestMeWithGarbageCookieIs401` (P2) (item 37) |
| Photo and icon uploads (multipart) | media and settings handlers, `MaxBytesReader` | 413, 415 or 422 problem JSON; nothing stored | media tests: `TestUploadMediaNotJPEGIs415`, `TestUploadMediaFullOver10MBIs413`, `TestUploadMediaRequestOver12MBIs413`, `TestUploadMediaNotMultipartIs415`, `TestUploadMediaMissingPartIs422`, `TestUploadMediaNoOwnerIs422`; icon tests (P2, item 44): `TestUploadIconNotPNGIs415`, `TestUploadIconPartOver1MBIs413`, `TestUploadIconRequestOver3MBIs413`, `TestUploadIconMissingPartIs422`, `TestUploadIconNotMultipartIs415`, `TestUploadIconOtherMultipartIs415`, `TestUploadIconMalformedMultipartIs422` |
| `.env` variables | platform config | `serve` exits non-zero with one message listing every bad variable | `TestLoadRejectsBadValues`, `TestLoadListsEveryMissingVariable` in the config package (P0) |
| CLI args | cmd/open-customer | exit 1 with a one-line stderr message | `TestRun` in cmd/open-customer (P0; each phase adds its subcommands; P2: `reset-password`, `setup-link` and `healthcheck` arity and missing config), plus `TestResetPasswordUnknownUser`, `TestResetPasswordMalformedUsername` and `TestSetupLinkWhenUserExists` |
| Backup status file | `internal/platform/backupstatus` (`Read`), used by the migration guard (P1, item 27) and status `BackupRed` (P4, item 60) | per the design doc's "Upgrades and the migration guard" and "Status screen" | status and migrate tests: missing, malformed, stale file (P1, items 27 and 60) |
| Stored attribute values (jsonb) | attributes `Validate` on write; typed decode on read | write: 422 `validation`; read: logs `code=`, the rest of the list still renders | attributes tests: `TestValidateUsesStoredDefinitions`, `TestValidateAcceptedValues`, `TestValidateRejectedValues` and `TestValidateRejectedWrites` (write), `TestFormatBadStoredValueLogsCode` (read) (P3, item 46) |
| Web Push service responses | push `SendAll` | counted as not delivered; never fails the daily timer | push tests: 410 and 5xx from a fake push service (P4, item 57) |
| API responses (web) | zod in the web API client | typed error shown to her, not "Offline" | `client.ts` test: a 200 body that fails its schema → `response_invalid` (P5, item 78) |
| Dexie rows (web cache, drafts) | zod in the web cache module | `clearCache()`, then refresh from `/bootstrap` | cache test: a bad stored row → cache cleared and refreshed (P5, item 78) |
| Push payload (service worker) | service worker `push` handler | a generic notification text | service worker test: malformed payload → generic text (P5, item 78) |
| Route params (web) | the route's load function | the not-found screen | route test: unknown id → not-found screen (P5, item 78) |
| Script args and env (deploy scripts) | top of each script | exit non-zero with a one-line message | a shell test per script: missing arg → non-zero exit (P7, item 86) |

- **Parse once, at the edge.** Outside input = request body, query, headers, env, config files,
  stored data, messages, CLI args, another process's output, an API response. Parse it where it
  enters, into a precise type. Sensor: Go typed structs + `DisallowUnknownFields` + the owning module
  validating each field; web zod; both checked by the compiler / svelte-check.
- **Fail closed.** Bad input is rejected (4xx, non-zero exit, a cache reset), never half accepted.
  Other work continues: one bad request or record never takes down the rest. Prose only; each
  boundary has a test with a bad input.
- **Past the boundary, the type is the proof.** No re-validation, no cast to force a value through.
  A cast lives only inside a small, named, tested helper. Sensor: typescript-eslint `no-explicit-any`
  (web); Go type assertions are prose only.
- **Log the error path and a code, never the value.** A schema error often contains the input (a
  client's name or phone, a token). Log `path` + `code` only. Prose only.
- **Unknown extra fields from systems you don't own pass through; malformed known fields fail.**
  Our own web client is owned (unknown fields are rejected); Web Push services and Cloudflare are
  not. Don't enum-lock a field a third party may extend. Prose only.
- **State is a tagged union with an exhaustive match**, not a set of booleans (appointment status,
  due-item kind, the client's Offline / error / ok result). A forgotten case is a red check. Sensor:
  golangci-lint `exhaustive` (Go); typescript-eslint `switch-exhaustiveness-check` (web).
