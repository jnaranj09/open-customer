import { describe, expect, it, vi } from 'vitest';
import type { Api } from './api/client';
import { browserPushDeps, type BrowserEnv } from './push-browser';

const api = {} as Pick<Api, 'pushKey' | 'subscribePush'>;

function env(over: Partial<BrowserEnv> = {}): BrowserEnv {
	return {
		hasServiceWorker: true,
		hasPushManager: true,
		notification: {
			permission: 'default',
			requestPermission: () => Promise.resolve('granted')
		},
		standaloneQuery: () => false,
		iosStandalone: undefined,
		ready: () => Promise.resolve({ pushManager: {} } as ServiceWorkerRegistration),
		...over
	};
}

describe('browserPushDeps', () => {
	it('is supported with service workers, push and notifications', () => {
		expect(browserPushDeps(api, env()).supported).toBe(true);
	});

	it('is unsupported without a push manager', () => {
		expect(browserPushDeps(api, env({ hasPushManager: false })).supported).toBe(false);
	});

	it('is unsupported without notifications', () => {
		expect(browserPushDeps(api, env({ notification: undefined })).supported).toBe(false);
	});

	it('is standalone by the display-mode query', () => {
		expect(browserPushDeps(api, env({ standaloneQuery: () => true })).standalone).toBe(true);
	});

	it('is standalone by the iOS flag', () => {
		expect(browserPushDeps(api, env({ iosStandalone: true })).standalone).toBe(true);
	});

	it('is not standalone in a browser tab', () => {
		expect(browserPushDeps(api, env({ iosStandalone: false })).standalone).toBe(false);
	});

	it('reads the permission each time', () => {
		const notification = {
			permission: 'default' as NotificationPermission,
			requestPermission: vi.fn()
		};
		const deps = browserPushDeps(api, env({ notification }));
		notification.permission = 'granted';
		expect(deps.permission()).toBe('granted');
	});

	it('reads default permission when notifications are missing', () => {
		expect(browserPushDeps(api, env({ notification: undefined })).permission()).toBe('default');
	});

	it('asks for permission through the browser', async () => {
		const requestPermission = vi.fn(() => Promise.resolve<NotificationPermission>('denied'));
		const deps = browserPushDeps(
			api,
			env({ notification: { permission: 'default', requestPermission } })
		);
		expect(await deps.requestPermission()).toBe('denied');
	});

	it('denies the request when notifications are missing', async () => {
		expect(await browserPushDeps(api, env({ notification: undefined })).requestPermission()).toBe(
			'denied'
		);
	});

	it('takes the push manager from the ready registration', async () => {
		const pushManager = {} as PushManager;
		const deps = browserPushDeps(
			api,
			env({ ready: () => Promise.resolve({ pushManager } as ServiceWorkerRegistration) })
		);
		expect((await deps.registration()).pushManager).toBe(pushManager);
	});
});
