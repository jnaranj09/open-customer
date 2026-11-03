# Critic Report: Stage 7 (round 5)

Target: `docs/designs/release-1.md`, FeatureRange lines 1–1218. Reviewers (fresh contexts, direct dispatch): `egm-design` and `egm-structural` (feature-spec-standard), `egm-style` (no standard).

Totals: 0 Blocker, 6 Major, 26 Nit.

Overlaps: design-N2 = comprehension #5 ("without q"); style-N3 = comprehension #16 (SINPE); style-N4 = comprehension #17.

## Design

Design (Stage 7, round 5): 0 Blocker, 1 Major, 10 Nit. Meets the standard; no owner decisions reopened.

M1 (110, 48, 158, 651, seed at 1859): Goal 7 fails against the starter content. starter seeds "SINPE Móvil" (Costa Rica only) from code. Fix: (a) pick starter payment options by country_code (one allowed regional starter table), or (b) reword Goal 7 to list the starter content as an editable named exception.

N1 (102 vs 747/752): "at most 24 h apart" vs the hourly check that skips when <24 h, so the real gap is up to ~25 h. Fix: skip at <23 h, or say 25 h.
N2 (681): leftover "without q".
N3 (554/498): the pending ¿Lo enviaste? is cleared when a "before" item expires (tap at 14:55, reopen at 15:05), so the sent message is never logged. Fix: still ask, and allow a log POST; or accept it.
N4 (289): the timer must record the day only after a successful count; on error, retry the next minute.
N5 (618, 753): photo upload order (write + sync the files, then insert the row; remove the files if the insert fails) is not stated.
N6 (799): the restore temp folder location and free space are not given; check space before dropdb.
N7 (584, 619): logout leaves photos in the HTTP cache (max-age 1 year). Soften the sentence, or send Clear-Site-Data: "cache".
N8 (615): createImageBitmap on 24–48 MP photos may run out of memory in the PWA. Add it to the iOS checks; keep a fallback.
N9 (695, 707): Excel mangles +506… phone numbers; formula injection (=, +, -, @).
N10: line 149 (permission needs a user tap: a button); 503 (show the count of late items that will disappear); 528 ("client's history" wording).

## Structural

Structural (Stage 7, round 5): 0 Blocker, 1 Major, 5 Nit. All required sections are present; headings nest correctly; tables and code blocks are well formed.

Major 1 (1166–1170): M1 has no specific reason. "Cons: None measured" plus "Author preference". Fix: name a concrete con (the author's Svelte experience, a smaller bundle, less boilerplate for one maintainer), or state that this is a preference with no technical edge and list it as an accepted trade-off.

Nit 1 (880, 912, 1070, 1102, 1148, 1170): the final labels vary (Reason for Rejection / Outcome / Author preference), and the intro does not explain them. Fix: one sentence at 880 that defines the three labels.
Nit 2 (291, 568, 1191): section references do not match the heading names. "Reminder rules" vs `Reminder rules and the "done" rule`; nested quotes at 291; Verification Plan is not a link. Fix: anchor links.
Nit 3 (851, 862): "handover checklist (item 83)" vs "pre-handover checklist in deploy/README.md". Fix: one name.
Nit 4 (8, 1184, 1195, 1217): horizontal rules are placed unevenly. Fix: keep only 8 and 1217.
Nit 5 (43): `**The two reminder kinds:**` has the colon inside the bold. Fix: `**The two reminder kinds**:`.

## Style

Style (Stage 7, round 5): 0 Blocker, 4 Major, 11 Nit.

M1 (482, 626): "she/her" refers to the client, which breaks the "she = owner" convention. Fix: "the client's".
M2 (941, 1122, 1154, 1181): "not verified" / "unverified" instead of "not checked". Fix: use "not checked".
M3 (20): "The app also reminds clients to come back". Wrong: the app reminds her. Fix: "tells her when to ask a client to come back for a refill (relleno)".
M4 (479): "chip" means both the row chip and the inline chip. Fix: "chip in the row" vs "inline chip".

N1 (20 vs 42): "fill" vs "refill".
N2 (728, 731): `<owner>` in the image path clashes with "owner". Fix: `<github-user>`.
N3 (158 vs 651): "SINPE" vs "SINPE Móvil".
N4 (218): "'Enviar', 'Omitir'" → "'Enviar' and 'Omitir'".
N5 (291): the same idea is said twice.
N6 (466/471): "she never types it" is said twice.
N7 (503): "Come-back items" (the rule applies to both kinds); "waiting as late".
N8 (637): "a rebook skip ... is lifted" is jargon.
N9 (516–526, 591–595, 751–756, 797–801): "First/Then/Finally" in numbered lists. (These words are the user's style rule for flows; ignore.)
N10 (1134, 1176, 1181): opinion-based reasons.
N11 (1057): "Nov 19, 2026" → "November 19, 2026".
