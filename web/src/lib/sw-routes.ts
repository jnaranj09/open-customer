// The pure decisions of the service worker (design item 65), kept out of `service-worker.ts` so
// Vitest can test them.

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
