# Critic Report: Stage 7 (round 4)

Target: `docs/designs/release-1.md`, FeatureRange lines 1–1196.
Reviewers (fresh contexts, direct dispatch), all completed:
- `egm-design` (feature-spec-standard)
- `egm-structural` (feature-spec-standard)
- `egm-style` (no standard)

Verdict: no Blockers, 9 Majors, 26 Nits.

Overlaps:
- design-n4 = structural-4-2 (the M1 reason);
- style-4-2 = Stage 6 #1 (owner vs author);
- structural-n2 = style-n8 (Cloudflare stated twice).

Synthesizer notes:
- design-4-1 and design-4-5 reopen the owner's D1 decision (read-only offline).
- structural-4-1 is mostly already handled: item 14 rewrites those links.

---

### Design (Stage 7): 0 Blocker, 5 Major, 6 Nit

All the gaps come from one source: the PC is sometimes off, and the iPhone flow does not fully handle it.

**design-4-1 (Major): with the server off, "Para enviar hoy" shows yesterday's list, and "Enviar" is disabled** (26, 206–215, 547–550, 566, 1000–1004).
- If the PC is off this morning, no "before" reminder goes out, and nothing tells her.
- Fix:
  - `/bootstrap` also returns the items due tomorrow (and maybe the day after), each with its due day.
  - Allow "Enviar" offline. The item then stays "Enviado, sin guardar: toca Sí cuando haya conexión", the same state as a failed POST.
  - Add this option to G2.
  - Label the cached list with its day.

**design-4-2 (Major): the "¿Lo enviaste?" confirmation is lost if iOS closes the app while she is in WhatsApp** (183–194, 535–545). Fix:
- Store the pending confirmation in Dexie with the time.
- On any app start, show the sheet for a pending item from the last hour.
- Mark the item "Abierto en WhatsApp 10:05".
- Add a test where the app is killed.

**design-4-3 (Major): the Home Screen badge is set once and never cleared** (286, 558). Fix: call `navigator.setAppBadge(count)` / `clearAppBadge()` after every bootstrap and every log POST. Add a test.

**design-4-4 (Major): no push on a zero-count day, so silence means either "PC off" or "nothing to send"** (286, 832, 839, 980–984). Fix:
- Always send the daily push ("Hoy no hay mensajes para enviar"), which also proves the PC is on and resets the badge;
- or list the ambiguity as an accepted risk.

**design-4-5 (Major): Goal 4 can fail just because the PC was off** (97–98, 213–214, 569–572).
- She can't start a booking offline, and drafts hold one per form.
- Fix:
  - "+" opens offline and saves a local draft labelled "Sin guardar";
  - drafts become a list;
  - or restate Goal 4 to say how downtime days count.

Nits:
- design-n1: username input: trim and lowercase it; `autocapitalize="off"`; `autocomplete` attributes for iCloud Keychain.
- design-n2: a Cloudflare challenge or WAF block shows as "server off". Turn Bot Fight Mode off or add a skip rule; show the last failed status on the status screen.
- design-n3: state that a rollback drops data entered after the upgrade. Upgrade outside her working hours.
- design-n4: M1 is rejected only by preference.
- design-n5: no "always-on mini-PC / VPS" alternative entry.
- design-n6: a 0-day "before" item is useless for appointments before the push hour. Document it and add a test row.

### Structural (Stage 7): 0 Blocker, 2 Major, 9 Nit

Every required section is present, and the heading levels are clean.

**structural-4-1 (Major): links break after P7** (6, 54, 858, 1177–1179). The reviewer says item 11 has no plan to rewrite them.
- **Contested by the synthesizer:** item 14 already has a table that rewrites every one of these links, plus a grep check, in the same commit.
- The real issue is that a reader of item 11 doesn't see it. Fix: a pointer from item 11 to item 14.

**structural-4-2 (Major): some rejected alternatives give no specific reason.**
- M1 (1144–1148) says only "the author chose to keep SvelteKit".
- H4 (1044–1048) says only "the author chose a USB disk".
- Fix: state the real reason, or file the entry as a preference/decision.

Nits:
- structural-n1 (825–840): rename to "Risks & Mitigations"; give each accepted risk a "Mitigation:" lead.
- structural-n2 (821, 838): the Cloudflare risk is stated twice.
- structural-n3 (1088–1112): J3 and J4 are filed under the wrong group.
- structural-n4 (288 and others): section references mix styles; 288 has nested quotes.
- structural-n5 (347–434): the shorthand block is tagged `sql`.
- structural-n6 (8, 1162, 1173, 1195): horizontal rules are used unevenly.
- structural-n7 (481, 670, 686, 696, 720, 754): no blank line before some lists.
- structural-n8 (129–136): Open Questions entries don't have the same fields (source, deadline).
- structural-n9 (1175–1193): Additional Resources and Future Improvements sit mid-document.

### Style (Stage 7): 0 Blocker, 2 Major, 11 Nit

**style-4-1 (Major): the doc breaks its own rule for naming the two reminder kinds** (42–43 vs 93, 131, 157, 495, 496, 957). Other names in use: "before the appointment" reminder, "refill reminder", "21-day refill rule", "Refills", "the nudge". Line 42 also makes "relleno"/"refill" a synonym. Fix: use "before" / "come-back" everywhere; keep "refill" only as the nail example.

**style-4-2 (Major): "Owner" means two people** (49 vs 1022, 1080, 1120). "The owner chose phone-only search in the round-3 review" means the author. "Round-3 review" is never defined. Fix: "the author" for design decisions, "the owner" or "she" for the business owner. Give the reason, not the meeting.

Nits:
- style-n1: "not checked", "not verified", "unverified" and "not measured" are mixed. Use "not measured" for numbers and "unverified" for behavior.
- style-n2 (566 vs 1016): the bootstrap size estimate is worded two ways.
- style-n3 (871, 1004, 1010, 1086, 1112, 1153–1154, 1160): opinion-based rejection reasons.
- style-n4 (1144–1148): M1 has no reason. Label it "author preference".
- style-n5 (890, 1080, 1126): "Reason for Rejection" on options that were kept. Use "Outcome" instead.
- style-n6 (583, 599, 600): the icon timing is stated three times.
- style-n7 (594 vs 1120): the username-and-password rationale is repeated.
- style-n8 (821 vs 838): "Cloudflare sees decrypted traffic" is stated twice.
- style-n9 (829, 495, 288): a filler sentence, a calendar analogy, a repeated clause.
- style-n10 (1035–1036): dated wording ("brand new", "a year in production").
- style-n11 (54, 933, 1022): "the review" is ambiguous.
