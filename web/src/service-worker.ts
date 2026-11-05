// The service worker (design item 65): the app shell works with the server off, the data never
// does. It handles no `/api/*`, manifest or icon request, so those always reach the network.
import { version } from '$app/env';
import { assets, immutable } from '$app/manifest';
import { self as sw } from '$app/service-worker';
import { pushShown, swAction } from './lib/sw-routes';

const CACHE = `oc-${version}`;
// The SPA fallback page is not in SvelteKit's lists, so it is added by hand.
const SHELL = '/index.html';
// SvelteKit lists paths without the leading slash ("_app/immutable/..."); the cache keys and the
// request paths have it.
const asPath = (file: { path: string }): string =>
	new URL(file.path, 'https://x.invalid/').pathname;
const PRECACHE = [...immutable.map(asPath), ...assets.map(asPath), SHELL];
const PRECACHED: ReadonlySet<string> = new Set(PRECACHE);
const SEND_PATH = '/enviar';

sw.addEventListener('install', (event) => {
	// No skipWaiting here: a new worker waits until she taps the update prompt (SKIP_WAITING).
	event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(PRECACHE)));
});

sw.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) =>
				Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key)))
			)
	);
});

sw.addEventListener('message', (event) => {
	const data: unknown = event.data;
	if (
		typeof data === 'object' &&
		data !== null &&
		(data as { type?: unknown }).type === 'SKIP_WAITING'
	) {
		void sw.skipWaiting();
	}
});

sw.addEventListener('fetch', (event) => {
	const request = event.request;
	const url = new URL(request.url);
	if (request.method !== 'GET' || url.origin !== sw.location.origin) {
		return;
	}
	const action = swAction(url, request.mode, PRECACHED);
	if (action === 'network') {
		return;
	}
	const key = action === 'shell' ? SHELL : url.pathname;
	event.respondWith(
		caches
			.open(CACHE)
			.then((cache) => cache.match(key))
			.then((hit) => hit ?? fetch(request))
	);
});

sw.addEventListener('push', (event) => {
	// Every push shows a notification, even one with no or a bad payload (see `pushShown`).
	const shown = pushShown(event.data ? event.data.text() : null);
	if (!shown.valid) {
		console.warn('code=push_payload_invalid');
	}
	const badge = shown.badge;
	const nav: { setAppBadge?: (n: number) => Promise<void> } = sw.navigator;
	event.waitUntil(
		Promise.all([
			sw.registration.showNotification(shown.title, { body: shown.body }),
			badge !== null && nav.setAppBadge ? nav.setAppBadge(badge).catch(() => undefined) : undefined
		])
	);
});

sw.addEventListener('notificationclick', (event) => {
	event.notification.close();
	event.waitUntil(
		sw.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windows) => {
			const open = windows[0];
			return open
				? open.focus().then((w) => w.navigate(SEND_PATH))
				: sw.clients.openWindow(SEND_PATH);
		})
	);
});
