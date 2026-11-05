// The real browser behind `PushDeps` (push.ts): the one place that reads `navigator`, `Notification`
// and the display mode for push. Tests pass a fake `BrowserEnv`.
import type { Api } from './api/client';
import type { PushDeps } from './push';

type NotificationLike = {
	readonly permission: NotificationPermission;
	requestPermission: () => Promise<NotificationPermission>;
};

export type BrowserEnv = {
	hasServiceWorker: boolean;
	hasPushManager: boolean;
	/** `undefined` where the browser has no Notification API (an iOS Safari tab). */
	notification: NotificationLike | undefined;
	/** `matchMedia('(display-mode: standalone)').matches`. */
	standaloneQuery: () => boolean;
	/** `navigator.standalone`, which only iOS sets. */
	iosStandalone: boolean | undefined;
	ready: () => Promise<ServiceWorkerRegistration>;
};

/** The environment of the page this runs in. */
function pageEnv(): BrowserEnv {
	return {
		hasServiceWorker: 'serviceWorker' in navigator,
		hasPushManager: 'PushManager' in window,
		notification: 'Notification' in window ? Notification : undefined,
		standaloneQuery: () => window.matchMedia('(display-mode: standalone)').matches,
		iosStandalone: (navigator as Navigator & { standalone?: boolean }).standalone,
		ready: () => navigator.serviceWorker.ready
	};
}

/** `PushDeps` for the page, or for a fake environment in tests. */
export function browserPushDeps(
	api: Pick<Api, 'pushKey' | 'subscribePush'>,
	env: BrowserEnv = pageEnv()
): PushDeps {
	return {
		api,
		supported: env.hasServiceWorker && env.hasPushManager && env.notification !== undefined,
		standalone: env.iosStandalone === true || env.standaloneQuery(),
		permission: () => env.notification?.permission ?? 'default',
		requestPermission: () => env.notification?.requestPermission() ?? Promise.resolve('denied'),
		registration: env.ready
	};
}
