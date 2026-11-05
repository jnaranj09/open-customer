import { describe, expect, it, vi } from 'vitest';
import type { Api } from './api/client';
import { activatePush, checkPush, type PushDeps } from './push';

const SUB_JSON = {
	endpoint: 'https://push.example.test/send/abc',
	expirationTime: null,
	keys: { p256dh: 'BPublicKeyAAAA', auth: 'AuthSecret' }
};
const KEY = 'AQIDBA'; // base64url of bytes 1,2,3,4
const SUB_ID = { id: '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f' };

function sub(json: unknown = SUB_JSON): PushSubscription {
	return { toJSON: () => json } as unknown as PushSubscription;
}

function setup(over: Partial<PushDeps> = {}, existing: PushSubscription | null = null) {
	const subscribe = vi.fn<PushManager['subscribe']>(() => Promise.resolve(sub()));
	const getSubscription = vi.fn(() => Promise.resolve(existing));
	const subscribePush = vi.fn<Api['subscribePush']>(() =>
		Promise.resolve({ ok: true, data: SUB_ID })
	);
	const pushKey = vi.fn<Api['pushKey']>(() =>
		Promise.resolve({ ok: true, data: { public_key: KEY } })
	);
	const requestPermission = vi.fn(() => Promise.resolve<NotificationPermission>('granted'));
	const deps: PushDeps = {
		api: { pushKey, subscribePush },
		supported: true,
		standalone: true,
		permission: () => 'granted',
		requestPermission,
		registration: () => Promise.resolve({ pushManager: { getSubscription, subscribe } }),
		...over
	};
	return { deps, subscribe, getSubscription, subscribePush, pushKey, requestPermission };
}

describe('checkPush', () => {
	it('re-posts an existing subscription', async () => {
		const { deps, subscribePush } = setup({}, sub());
		expect(await checkPush(deps)).toBe('active');
		expect(subscribePush).toHaveBeenCalledWith(SUB_JSON);
	});

	it('is needs_activation without permission', async () => {
		const { deps, subscribePush } = setup({ permission: () => 'default' }, sub());
		expect(await checkPush(deps)).toBe('needs_activation');
		expect(subscribePush).not.toHaveBeenCalled();
	});

	it('is needs_activation with permission but no subscription', async () => {
		const { deps } = setup({}, null);
		expect(await checkPush(deps)).toBe('needs_activation');
	});

	it('is not_installed in a Safari tab', async () => {
		const { deps, getSubscription } = setup({ standalone: false, supported: false }, sub());
		expect(await checkPush(deps)).toBe('not_installed');
		expect(getSubscription).not.toHaveBeenCalled();
	});

	it('is unsupported in the installed app when the browser has no push', async () => {
		const { deps } = setup({ supported: false });
		expect(await checkPush(deps)).toBe('unsupported');
	});

	it('stays active when the re-post fails, so the next open tries again', async () => {
		const { deps, subscribePush } = setup({}, sub());
		subscribePush.mockResolvedValue({ ok: false, error: { kind: 'offline' } });
		expect(await checkPush(deps)).toBe('active');
	});

	it('is needs_activation when the browser gives a subscription that is not valid JSON', async () => {
		const { deps, subscribePush } = setup({}, sub({ endpoint: 5 }));
		expect(await checkPush(deps)).toBe('needs_activation');
		expect(subscribePush).not.toHaveBeenCalled();
	});

	it('is needs_activation when reading the subscription throws', async () => {
		const { deps } = setup({ registration: () => Promise.reject(new Error('no worker')) });
		expect(await checkPush(deps)).toBe('needs_activation');
	});
});

describe('activatePush', () => {
	it('stores the subscription with expirationTime (the JSON the browser gives)', async () => {
		const { deps, subscribePush } = setup({ permission: () => 'default' });
		const result = await activatePush(deps);
		expect(result.ok).toBe(true);
		expect(subscribePush).toHaveBeenCalledWith(SUB_JSON);
	});

	it('subscribes with the server key decoded from base64url', async () => {
		const { deps, subscribe } = setup({ permission: () => 'default' });
		await activatePush(deps);
		expect(subscribe).toHaveBeenCalledWith({
			userVisibleOnly: true,
			applicationServerKey: new Uint8Array([1, 2, 3, 4])
		});
	});

	it('asks for permission before it calls the server', async () => {
		const order: string[] = [];
		const { deps, pushKey, requestPermission } = setup({ permission: () => 'default' });
		requestPermission.mockImplementation(() => {
			order.push('permission');
			return Promise.resolve('granted');
		});
		pushKey.mockImplementation(() => {
			order.push('key');
			return Promise.resolve({ ok: true, data: { public_key: KEY } });
		});
		await activatePush(deps);
		expect(order).toEqual(['permission', 'key']);
	});

	it('does not ask again when permission is already granted', async () => {
		const { deps, requestPermission } = setup();
		await activatePush(deps);
		expect(requestPermission).not.toHaveBeenCalled();
	});

	it('fails and stores nothing when she denies the permission', async () => {
		const { deps, requestPermission, subscribePush } = setup({ permission: () => 'default' });
		requestPermission.mockResolvedValue('denied');
		const result = await activatePush(deps);
		expect(result).toEqual({ ok: false, error: { kind: 'push_unavailable' } });
		expect(subscribePush).not.toHaveBeenCalled();
	});

	it('fails in a Safari tab without asking for permission', async () => {
		const { deps, requestPermission } = setup({ standalone: false });
		const result = await activatePush(deps);
		expect(result).toEqual({ ok: false, error: { kind: 'push_unavailable' } });
		expect(requestPermission).not.toHaveBeenCalled();
	});

	it('returns the key error when the server key cannot be read', async () => {
		const { deps, pushKey, subscribe } = setup();
		pushKey.mockResolvedValue({ ok: false, error: { kind: 'offline' } });
		expect(await activatePush(deps)).toEqual({ ok: false, error: { kind: 'offline' } });
		expect(subscribe).not.toHaveBeenCalled();
	});

	it('fails when the browser refuses to subscribe', async () => {
		const { deps, subscribe } = setup();
		subscribe.mockRejectedValue(new Error('denied'));
		expect(await activatePush(deps)).toEqual({ ok: false, error: { kind: 'push_unavailable' } });
	});

	it('returns the server error when it refuses the subscription', async () => {
		const { deps, subscribePush } = setup();
		subscribePush.mockResolvedValue({ ok: false, error: { kind: 'offline' } });
		expect(await activatePush(deps)).toEqual({ ok: false, error: { kind: 'offline' } });
	});
});
