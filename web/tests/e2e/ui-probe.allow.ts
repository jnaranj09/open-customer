// The known findings of `ui-probe.spec.ts`, each waiting on a fix. `bug` is the finding's id
// (`UIP-` and the id it had when it was found: M and L from the release-1 bug hunt, N from this
// probe) and `why` says what the bug is. Only shrinks: an entry that matches nothing on the screen
// it is scoped to fails, so a fix removes its entries in the same change. A new entry needs the
// owner's yes.
import type { Allow } from './probe';

/** Empty: every finding found so far is fixed. An entry is `{ route, check, state?, width?,
 *  target?, why, bug }` (see `Allow` in probe.ts). */
export const ALLOW: readonly Allow[] = [];
