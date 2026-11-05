// Asks the server who she is, for the guard (design doc item 71).
import type { Api } from './api/client';
import type { Session } from './guard';
import type { Me } from './api/schemas';

/** Online only when the server answered clearly: "needs setup", "no session" (401) or "this is
 *  her" (also with a forced password change). Any other failure (no answer, an answer that is not understood) is `offline`, which the
 *  guard never redirects on, so a hiccup cannot log her out of the cached app. */
export async function resolveSession(
	api: Pick<Api, 'me' | 'setupStatus'>,
	cachedMe: Me | null
): Promise<Session> {
	const offline: Session = { kind: 'offline', cachedMe };
	const status = await api.setupStatus();
	// A session that must change its password gets 403 on every route but /me, this one included:
	// the answer says she is signed in, so /me tells the rest.
	const forced =
		!status.ok && status.error.kind === 'problem' && status.error.type === 'must-change-password';
	if (!status.ok && !forced) {
		return offline;
	}
	if (status.ok && status.data.needs_setup) {
		return { kind: 'online', setupNeeded: true, me: null };
	}
	const me = await api.me();
	if (me.ok) {
		return { kind: 'online', setupNeeded: false, me: me.data };
	}
	return me.error.kind === 'unauthorized'
		? { kind: 'online', setupNeeded: false, me: null }
		: offline;
}
