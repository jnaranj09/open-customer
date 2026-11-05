// Asks the server who she is, for the guard (design doc item 71).
import type { Api } from './api/client';
import type { Session } from './guard';
import type { Me } from './api/schemas';

/** Online only when the server answered clearly: "needs setup", "no session" (401) or "this is
 *  her". Any other failure (no answer, an answer that is not understood) is `offline`, which the
 *  guard never redirects on, so a hiccup cannot log her out of the cached app. */
export async function resolveSession(
	api: Pick<Api, 'me' | 'setupStatus'>,
	cachedMe: Me | null
): Promise<Session> {
	const offline: Session = { kind: 'offline', cachedMe };
	const status = await api.setupStatus();
	if (!status.ok) {
		return offline;
	}
	if (status.data.needs_setup) {
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
