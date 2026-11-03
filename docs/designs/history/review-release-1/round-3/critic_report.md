# Critic Report: Stage 7 (round 3)

Target: `docs/designs/release-1.md`, FeatureRange lines 1–1172.

Reviewers (fresh contexts, direct dispatch), all completed:
- `egm-design` (feature-spec-standard)
- `egm-structural` (feature-spec-standard)
- `egm-style` (no standard)

Verdict: no Blockers, 6 Major, 26 Nit. A few findings overlap: design-n4 = structural-3-2 (the `restore` service), and style-n8 = structural-n3 (the "item N" links).

---

### Design (Stage 7): 0 Blocker, 3 Major, 6 Nit

**design-3-1 (Major): a new or edited come-back rule fills the list with old "late" messages** (482–484, 498, 505–507).
- The `created_at` guard looks only at when the appointment was created, not when the rule was created or changed.
- Three actions bring back every visit in the 60-day look-back as a late item:
  - adding a rule to a service that already has visits;
  - lowering `offset_days`;
  - giving an archived-template rule a new template.
- Fix:
  - Add `reminder_rules.effective_at`. Set it on create, and when `offset_days` or `template_id` changes.
  - An item exists only if its due day is on or after that day.
  - Add a matching test row.

**design-3-2 (Major): backup and push failures show only on a screen she has to open** (677–686, 735, 791, 831–832).
- Nothing takes her to "Estado del sistema", and the author has no alert. Backups could stop for weeks before anyone sees it.
- Fix:
  - Show the red state where she already looks: a banner or dot on "Hoy" or on Ajustes ("Respaldo con problema — avisa a <owner>").
  - Optionally, the backup container also notifies the author.

**design-3-3 (Major): "She keeps her data through the export" holds only if she exported before the PC broke** (814; Goal 5; H8).
- The export needs the server. The USB backups are encrypted with a password only the author has.
- Fix: a monthly nudge on "Hoy" ("Descarga tus datos (última: hace 32 días)"), or rewrite the accepted risk honestly. Optionally, she keeps a sealed copy of the restic password.

Nits:
- design-n1 (207, 554): no signal on her phone shows "El sistema está apagado". Use `navigator.onLine` or the error type to show "Sin conexión a internet" instead.
- design-n2 (527, 535): after a failed "Sí", the item looks unsent again, so she may send the message twice. Keep a local "Enviado, falta confirmar" mark.
- design-n3 (482, 487, 936–940): any later visit cancels a refill, even an unrelated pedicure. Write this down in E5 and add it to the interview questions.
- design-n4 (106 vs 757): "four services" leaves out the profile-only `restore` service.
- design-n5 (96 vs 714): Goal 5's "every day the PC is on" does not match the rolling 24 h check.
- design-n6 (755, 764, 1147): after a restore to an older version, the phone holds a newer SPA. State that the version prompt also covers going down a version.
- Optional: there is no "small VPS / managed host" alternative entry.

### Structural (Stage 7): 0 Blocker, 2 Major, 8 Nit

The heading levels are clean, and every section the standard asks for is present. The links still work today; item 14 rewrites them when the files move.

**structural-3-1 (Major): choices kept from the SAD have no alternatives here** (36, 838–1136).
- These ADRs are kept, but the options they rejected appear nowhere in this document: 001 Go, 003 pgx + sqlc, 005 modular binary, 007 goose, 012 Postgres sessions, 013 own renderer, 016 Web Push.
- Once the SAD moves into history, a new reader sees the core stack with no rejected options.
- Fix: add a "Kept from the SAD" group with one row per ADR (choice, rejected options, one-line reason), or link to the ADR table in `history/sad-original.md`.

**structural-3-2 (Major): the Deployment table and Goal 8 leave out the `restore` service** (106, 690–701, 758–760, item 81).
- Fix: add a `restore` row to the table. Reword Goal 8 to "four always-on services plus an on-demand `restore` profile".

Nits:
- structural-n1 (810–836): the accepted risks are a loose list. Make them a table (Risk, Why accepted, Partial mitigation, When to revisit) and consider "Risks & Mitigations". The intro says "no new mitigation", yet some entries list one.
- structural-n2 (8, 808, 838, 1138–1171): sections are in an odd order. Move the NFRs near Goals or the Technical Plan, and put Future Improvements before Additional Resources. Horizontal rules are used unevenly.
- structural-n3 (36, 283, 325–330, …): about 20 "item N" references have no link. Line 283 names a heading that does not match ("Done" rule).
- structural-n4 (84–91): the Goal 3 summary bullet is a sibling of the checks it summarizes.
- structural-n5 (194, 228): CUJ 5 and CUJ 8 break the Given/When/Then layout. CUJ 8's "When" has an implementation detail (`download`).
- structural-n6 (339–428): the data model block is tagged `sql` but is shorthand.
- structural-n7 (471, 654, 670, 679, 703, 737): no blank line before some lists.
- structural-n8 (1064–1088): J3 (WebP) and J4 (unofficial WhatsApp libraries) sit under "API and client stack".

### Style (Stage 7): 0 Blocker, 1 Major, 12 Nit

**style-3-1 (Major): the M1 rejection rests on claims the document marks as not measured** (1122–1124). The rejection gives "smaller first load" and "less code per screen", but neither was measured. The real reason is the author's choice. Fix: lead with the author's preference; mark the size claims "not measured", or drop them.

Nits:
- style-n1 (840): "Each alternative was rejected" does not hold for A5, I1 and K2. Fix: "rejected or narrowed".
- style-n2 (1096): "the owner approved the change" is ambiguous, because Terms define "Owner" as the business owner. Fix: "the author approved".
- style-n3 (799): "costs little now" is unmeasured; the priority is quoted by the wrong name. Fix: "This follows priority 5 (generic and self-hostable)."
- style-n4 (1088, 1135, 1130, 992, 986, 1062, 1116): subjective wording in the Alternatives rationales. Fix: state the concrete consequence or the size behind each one.
- style-n5 (615): "easy to add" is subjective. Fix: "can be added then without changing existing data".
- style-n6 (283): "tidy job" is undefined jargon.
- style-n7 (281, 554, 562, 711): paragraphs too dense to scan. Fix: split into sub-bullets, one rule each.
- style-n8 (36, 325, 341, 632, 654, 763): "item N" cross-references are formatted inconsistently. Fix: link the first one, or define "item N" in Terms.
- style-n9 (1166): "FR-I1 to FR-I5" lacks the "PRD" prefix used elsewhere.
- style-n10 (50): "EGM" is defined but never used.
- style-n11 (99, 101): "1 s" vs "1 second".
- style-n12 (907): the D3 Cons sentence is wordy.
