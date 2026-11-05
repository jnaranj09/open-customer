// The pure decisions of the service worker (design item 65), kept out of `service-worker.ts` so
// Vitest can test them.
import { t } from './i18n';

export type SwAction = 'shell' | 'precache' | 'network';

// Never cached: the wizard and settings change the manifest and the icons.
const NETWORK_PREFIXES = ['/api/', '/icons/'];
const NETWORK_PATHS = new Set(['/api', '/manifest.webmanifest', '/apple-touch-icon.png']);

/** What the worker does with a same-origin GET. `precached` holds the paths it stored on install.
 *  `network` means the worker does not answer: the browser goes to the network by itself. */
export function swAction(url: URL, mode: RequestMode, precached: ReadonlySet<string>): SwAction {
	const path = url.pathname;
	if (NETWORK_PATHS.has(path) || NETWORK_PREFIXES.some((prefix) => path.startsWith(prefix))) {
		return 'network';
	}
	if (precached.has(path)) {
		return 'precache';
	}
	return mode === 'navigate' ? 'shell' : 'network';
}

export type PushNotice = { title: string; body: string; count: number };

/** The due count in a push payload, or null when the payload is bad. */
export function pushCount(data: unknown): number | null {
	if (typeof data !== 'object' || data === null) {
		return null;
	}
	const { count } = data as Record<string, unknown>;
	return typeof count === 'number' && Number.isInteger(count) && count >= 0 ? count : null;
}

/** Parses the push payload `{title, body, count, url}` (design "Daily push"). Bad input is null. */
export function pushNotice(data: unknown): PushNotice | null {
	const count = pushCount(data);
	const { title, body } = (data ?? {}) as Record<string, unknown>;
	if (count === null || typeof title !== 'string' || typeof body !== 'string') {
		return null;
	}
	return { title, body, count };
}

/** What the worker shows for one push. */
export type PushShown = { title: string; body: string; badge: number | null; valid: boolean };

/** Parses the push payload text (null when the push carried none). Every push shows a
 *  notification: the page subscribes with `userVisibleOnly`, and iOS can revoke a subscription
 *  whose push shows none. An empty or bad payload (for example from a newer server than this
 *  worker) shows the fixed fallback text, sets no badge and is `valid: false`, so the worker logs
 *  a code. */
export function pushShown(text: string | null): PushShown {
	const notice = pushNotice(text === null ? null : parseJson(text));
	if (notice === null) {
		return {
			title: t('push_fallback_title'),
			body: t('push_fallback_body'),
			badge: null,
			valid: false
		};
	}
	return { title: notice.title, body: notice.body, badge: notice.count, valid: true };
}

function parseJson(text: string): unknown {
	try {
		return JSON.parse(text);
	} catch {
		// Not JSON reads as no payload: the caller shows the fallback and logs the code.
		return null;
	}
}
