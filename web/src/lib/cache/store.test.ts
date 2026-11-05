import 'fake-indexeddb/auto';
import Dexie from 'dexie';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Api } from '../api/client';
import type { Bootstrap } from '../api/schemas';
import type { Result } from '../api/errors';
import { DB_NAME, clearCache, writeSnapshot } from './db';
import { bootstrap } from './fixtures';
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

describe('the cache read model', () => {
	it('starts empty and online in the app instance', () => {
		expect([appCache.snapshot, appCache.offline]).toEqual([null, false]);
	});

	it('asks for a refresh when nothing is cached yet', async () => {
		const { cache, bootstrapCall } = setup({ ok: true, data: bootstrap(1, 'Ana Rojas') });
		await cache.reload();
		expect(bootstrapCall).toHaveBeenCalledOnce();
		expect(cache.snapshot?.clients.map((c) => c.name)).toEqual(['Ana Rojas']);
		expect(cache.meta.lastSyncAt).toBe('2026-11-04T12:00:00.000Z');
	});

	it('reads what is cached without calling the server', async () => {
		await writeSnapshot(bootstrap(0, 'Lucía Rojas'), '2026-11-03T12:00:00Z');
		const { cache, bootstrapCall } = setup({ ok: true, data: bootstrap() });
		await cache.reload();
		expect(bootstrapCall).not.toHaveBeenCalled();
		expect(cache.snapshot?.clients.map((c) => c.name)).toEqual(['Lucía Rojas']);
	});

	it('a cached row that fails its schema clears the cache and asks for a refresh', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		await writeSnapshot(bootstrap(0, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		const raw = new Dexie(DB_NAME);
		await raw.open();
		await raw.table('clients').toCollection().modify({ name: 42 });
		raw.close();
		const { cache, bootstrapCall } = setup({ ok: true, data: bootstrap(0, 'Marta Solano') });
		await cache.reload();
		expect(bootstrapCall).toHaveBeenCalledOnce();
		expect(cache.snapshot?.clients.map((c) => c.name)).toEqual(['Marta Solano']);
	});

	it('is offline after a refresh the server did not answer, and keeps the cached data', async () => {
		await writeSnapshot(bootstrap(0, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		const { cache } = setup({ ok: false, error: { kind: 'offline' } });
		await cache.reload();
		await cache.refresh();
		expect(cache.offline).toBe(true);
		expect(cache.snapshot?.clients.map((c) => c.name)).toEqual(['Ana Rojas']);
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
});
