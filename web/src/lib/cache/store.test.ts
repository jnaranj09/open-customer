import 'fake-indexeddb/auto';
import Dexie from 'dexie';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Api } from '../api/client';
import type { Bootstrap } from '../api/schemas';
import type { Result } from '../api/errors';
import { DB_NAME, clearCache, writeSnapshot } from './db';
import { guard } from '../guard';
import { resolveSession } from '../session';
import { bootstrap } from './fixtures';
import type { Badge } from './refresh';
import { cache as appCache, createCache } from './store.svelte';

const NOW = new Date('2026-11-04T12:00:00Z');

/** `answer` is what every call gets; `first` are answers for the first calls, in order. */
function setup(answer: Result<Bootstrap>, ...first: Result<Bootstrap>[]) {
	const bootstrapCall = vi.fn<Api['bootstrap']>(() => Promise.resolve(answer));
	for (const one of first) {
		bootstrapCall.mockResolvedValueOnce(one);
	}
	const cache = createCache({
		api: { bootstrap: bootstrapCall } as unknown as Api,
		now: () => NOW,
		badge: { set: () => Promise.resolve(), clear: () => Promise.resolve() }
	});
	return { cache, bootstrapCall };
}

beforeEach(async () => {
	await clearCache();
});

const FIRST = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f:0';
const SECOND = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f:1';

/** Breaks every cached client row behind the store's back, so its next read fails the schema. */
async function breakClients(): Promise<void> {
	const raw = new Dexie(DB_NAME);
	await raw.open();
	await raw.table('clients').toCollection().modify({ name: 42 });
	raw.close();
}

describe('the cache read model', () => {
	it('starts empty and online in the app instance', () => {
		expect([appCache.snapshot, appCache.offline]).toEqual([null, false]);
	});

	it('a bad cached row while she is signed in clears the cache and refreshes from the server', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		await writeSnapshot(bootstrap(0, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		await breakClients();
		const { cache, bootstrapCall } = setup({ ok: true, data: bootstrap(0, 'Marta Solano') });
		cache.setSyncing(true);
		await cache.load();
		await vi.waitFor(() => {
			expect(cache.snapshot?.clients.map((c) => c.name)).toEqual(['Marta Solano']);
		});
		expect(bootstrapCall).toHaveBeenCalledOnce();
	});

	it('a bad cached row with no session clears the cache and does not call the server', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		await writeSnapshot(bootstrap(0, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		await breakClients();
		const { cache, bootstrapCall } = setup({ ok: true, data: bootstrap(0, 'Marta Solano') });
		await cache.load();
		expect([cache.snapshot, bootstrapCall.mock.calls.length]).toEqual([null, 0]);
	});

	it('is offline after a refresh the server did not answer, and keeps the cached data', async () => {
		await writeSnapshot(bootstrap(0, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: false, error: { kind: 'offline' } });
		await cache.load();
		await cache.refresh();
		expect(cache.offline).toBe(true);
		expect(cache.snapshot?.clients.map((c) => c.name)).toEqual(['Ana Rojas']);
	});

	it('keeps a logged item hidden while the refresh after it fails', async () => {
		await writeSnapshot(bootstrap(1), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: false, error: { kind: 'offline' } });
		await cache.load();
		await cache.markLogged('a', { clearPending: false });
		await cache.refresh();
		expect(cache.logged).toEqual(['a']);
	});

	it('hides a logged item from the due list it shows', async () => {
		await writeSnapshot(bootstrap(2), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: false, error: { kind: 'offline' } });
		await cache.load();
		await cache.markLogged(FIRST, { clearPending: false });
		expect(cache.shownDue.map((i) => i.id)).toEqual([SECOND]);
	});

	it('a logged item stays hidden after the store is reopened', async () => {
		await writeSnapshot(bootstrap(2), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: false, error: { kind: 'offline' } });
		await cache.load();
		await cache.markLogged(FIRST, { clearPending: false });
		const reopened = setup({ ok: false, error: { kind: 'offline' } }).cache;
		await reopened.load();
		expect(reopened.shownDue.map((i) => i.id)).toEqual([SECOND]);
	});

	it('a refresh that no longer lists a logged item drops its stored id', async () => {
		await writeSnapshot(bootstrap(2), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: true, data: bootstrap(1) });
		await cache.load();
		await cache.markLogged(FIRST, { clearPending: false });
		await cache.markLogged(SECOND, { clearPending: false });
		await cache.refresh();
		expect(cache.meta.logged).toEqual([FIRST]);
	});

	it('a refresh asked before "Sí" that answers with the old list keeps the item hidden', async () => {
		let answer: (r: Result<Bootstrap>) => void = () => undefined;
		const bootstrapCall = vi.fn<Api['bootstrap']>(
			() =>
				new Promise<Result<Bootstrap>>((resolve) => {
					answer = resolve;
				})
		);
		const cache = createCache({
			api: { bootstrap: bootstrapCall } as unknown as Api,
			now: () => NOW,
			badge: { set: () => Promise.resolve(), clear: () => Promise.resolve() }
		});
		const focus = cache.refresh();
		await vi.waitFor(() => {
			expect(bootstrapCall).toHaveBeenCalledOnce();
		});
		await cache.markLogged(FIRST, { clearPending: false });
		answer({ ok: true, data: bootstrap(2) });
		await focus;
		expect(cache.shownDue.map((i) => i.id)).toEqual([SECOND]);
	});

	it('the badge after a refresh asked before "Sí" leaves out the item', async () => {
		let answer: (r: Result<Bootstrap>) => void = () => undefined;
		const set = vi.fn<Badge['set']>(() => Promise.resolve());
		const bootstrapCall = vi.fn<Api['bootstrap']>(
			() =>
				new Promise<Result<Bootstrap>>((resolve) => {
					answer = resolve;
				})
		);
		const cache = createCache({
			api: { bootstrap: bootstrapCall } as unknown as Api,
			now: () => NOW,
			badge: { set, clear: () => Promise.resolve() }
		});
		const focus = cache.refresh();
		await vi.waitFor(() => {
			expect(bootstrapCall).toHaveBeenCalledOnce();
		});
		await cache.markLogged(FIRST, { clearPending: false });
		answer({ ok: true, data: bootstrap(2) });
		await focus;
		expect(set).toHaveBeenCalledWith(1);
	});

	it('the badge after a refresh leaves out a logged item the list still holds', async () => {
		const set = vi.fn<Badge['set']>(() => Promise.resolve());
		const cache = createCache({
			api: {
				bootstrap: () => Promise.resolve({ ok: true, data: bootstrap(2) })
			} as unknown as Api,
			now: () => NOW,
			badge: { set, clear: () => Promise.resolve() }
		});
		await cache.markLogged(FIRST, { clearPending: false });
		await cache.refresh();
		expect(set).toHaveBeenCalledWith(1);
	});

	it('forgets logged items once a refresh succeeds', async () => {
		const { cache } = setup({ ok: true, data: bootstrap(0) });
		await cache.markLogged('a', { clearPending: false });
		await cache.refresh();
		expect(cache.logged).toEqual([]);
	});

	it('a server error after being offline is not offline, and the error is visible', async () => {
		const { cache } = setup(
			{ ok: false, error: { kind: 'response_invalid', path: '/bootstrap' } },
			{ ok: false, error: { kind: 'offline' } }
		);
		await cache.refresh();
		await cache.refresh();
		expect(cache.offline).toBe(false);
		expect(cache.lastError).toEqual({ kind: 'response_invalid', path: '/bootstrap' });
	});

	it('a failed cache write is recorded as an error, not a rejection', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const broken = bootstrap();
		broken.clients = broken.clients.map((c) => ({ ...c, attrs: { x: () => 1 } }));
		const { cache } = setup({ ok: true, data: broken });
		const result = await cache.refresh();
		expect(result).toEqual({ ok: false, error: { kind: 'cache_write_failed' } });
		expect(cache.lastError).toEqual({ kind: 'cache_write_failed' });
		expect(cache.offline).toBe(false);
	});

	it('logs a failed cache write by code only', async () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const broken = bootstrap();
		broken.clients = broken.clients.map((c) => ({ ...c, attrs: { x: () => 1 } }));
		const { cache } = setup({ ok: true, data: broken });
		await cache.refresh();
		expect(warn).toHaveBeenCalledWith('code=cache_write_failed');
	});

	it('clears the error after a refresh that works', async () => {
		const { cache } = setup(
			{ ok: true, data: bootstrap() },
			{ ok: false, error: { kind: 'offline' } }
		);
		await cache.refresh();
		await cache.refresh();
		expect(cache.lastError).toBeNull();
	});

	it('is online again after a refresh that works', async () => {
		const { cache } = setup({ ok: true, data: bootstrap() });
		await cache.refresh();
		expect(cache.offline).toBe(false);
	});

	it('never runs two refreshes at once, and the later call sees the later answer', async () => {
		const gates: Array<(r: Result<Bootstrap>) => void> = [];
		let active = 0;
		let overlapped = false;
		const bootstrapCall = vi.fn<Api['bootstrap']>(() => {
			active += 1;
			overlapped ||= active > 1;
			return new Promise<Result<Bootstrap>>((resolve) => {
				gates.push((r) => {
					active -= 1;
					resolve(r);
				});
			});
		});
		const cache = createCache({
			api: { bootstrap: bootstrapCall } as unknown as Api,
			now: () => NOW,
			badge: { set: () => Promise.resolve(), clear: () => Promise.resolve() }
		});
		const open = cache.refresh();
		const focus = cache.refresh();
		await vi.waitFor(() => {
			expect(gates).toHaveLength(1);
		});
		gates[0]?.({ ok: true, data: bootstrap(0, 'Ana Rojas') });
		await open;
		await vi.waitFor(() => {
			expect(gates).toHaveLength(2);
		});
		gates[1]?.({ ok: true, data: bootstrap(0, 'Marta Solano') });
		await focus;
		expect([overlapped, cache.snapshot?.clients.map((c) => c.name)]).toEqual([
			false,
			['Marta Solano']
		]);
	});

	it('load reads the cache without calling the server', async () => {
		await writeSnapshot(bootstrap(0, 'Lucía Rojas'), '2026-11-03T12:00:00Z');
		const { cache, bootstrapCall } = setup({ ok: true, data: bootstrap() });
		await cache.load();
		expect([bootstrapCall.mock.calls.length, cache.meta.lastSyncAt]).toEqual([
			0,
			'2026-11-03T12:00:00Z'
		]);
	});

	it('keeps the user it is told about, next to the synced data', async () => {
		await writeSnapshot(bootstrap(0, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: true, data: bootstrap() });
		const me = {
			id: '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f',
			username: 'ana',
			must_change_password: false
		};
		await cache.rememberMe(me);
		await cache.load();
		expect([cache.meta.me, cache.meta.lastSyncAt]).toEqual([me, '2026-11-03T12:00:00Z']);
	});

	it('after a clear, an offline session check finds no user and shows the offline text', async () => {
		await writeSnapshot(bootstrap(0, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: true, data: bootstrap() });
		const me = {
			id: '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f',
			username: 'ana',
			must_change_password: false
		};
		await cache.rememberMe(me);
		await cache.load();
		await clearCache();
		const offline = { ok: false, error: { kind: 'offline' } } as const;
		const session = await resolveSession(
			{ me: () => Promise.resolve(offline), setupStatus: () => Promise.resolve(offline) },
			cache.meta.me
		);
		expect([cache.snapshot, guard('/', session)]).toEqual([null, { kind: 'offline_blank' }]);
	});
});
