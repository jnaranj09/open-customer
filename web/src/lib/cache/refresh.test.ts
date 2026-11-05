import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Api } from '../api/client';
import type { Bootstrap } from '../api/schemas';
import type { Result } from '../api/errors';
import { clearCache, readAll, readMeta, writeSnapshot } from './db';
import { bootstrap } from './fixtures';
import { bootstrapWindow, refresh, shouldRefreshOnFocus, type Badge } from './refresh';

const NOW = new Date('2026-11-04T12:00:00Z');

function setup(answer: Result<Bootstrap>) {
	const bootstrapCall = vi.fn<Api['bootstrap']>(() => Promise.resolve(answer));
	const badge = {
		set: vi.fn<Badge['set']>(() => Promise.resolve()),
		clear: vi.fn<Badge['clear']>(() => Promise.resolve())
	};
	const deps = { api: { bootstrap: bootstrapCall } as unknown as Api, now: () => NOW, badge };
	return { deps, bootstrapCall, badge };
}

const ok = (data: Bootstrap): Result<Bootstrap> => ({ ok: true, data });

beforeEach(async () => {
	await clearCache();
});

describe('bootstrapWindow', () => {
	it('runs from 60 days back to 120 days ahead', () => {
		expect(bootstrapWindow('2026-11-04')).toEqual({ from: '2026-09-05', to: '2027-03-04' });
	});
});

describe('shouldRefreshOnFocus', () => {
	it('is false at 30 s', () => {
		expect(shouldRefreshOnFocus(1_000, 31_000)).toBe(false);
	});

	it('is true at 31 s', () => {
		expect(shouldRefreshOnFocus(1_000, 32_000)).toBe(true);
	});

	it('is true when there was no refresh yet', () => {
		expect(shouldRefreshOnFocus(null, 32_000)).toBe(true);
	});
});

describe('refresh', () => {
	it('stores the tables and lastSyncAt', async () => {
		const { deps } = setup(ok(bootstrap(0, 'Mariana Solís')));
		const result = await refresh(deps);
		expect(result).toEqual({ ok: true, data: undefined });
		expect((await readAll())?.clients.map((c) => c.name)).toEqual(['Mariana Solís']);
		expect((await readMeta()).lastSyncAt).toBe('2026-11-04T12:00:00.000Z');
	});

	it('after a refresh with 3 items the badge is 3', async () => {
		const { deps, badge } = setup(ok(bootstrap(3)));
		await refresh(deps);
		expect(badge.set).toHaveBeenCalledWith(3);
	});

	it('a refresh with 0 items clears the badge', async () => {
		const { deps, badge } = setup(ok(bootstrap(0)));
		await refresh(deps);
		expect(badge.clear).toHaveBeenCalledOnce();
		expect(badge.set).not.toHaveBeenCalled();
	});

	it('asks for the window around the business day, not the browser day', async () => {
		await writeSnapshot(
			{ ...bootstrap(0), settings: { ...bootstrap(0).settings, timezone: 'Pacific/Kiritimati' } },
			'2026-11-04T00:00:00Z'
		);
		// 12:00Z is already 5 Nov at +14:00.
		const { deps, bootstrapCall } = setup(ok(bootstrap(0)));
		await refresh(deps);
		expect(bootstrapCall).toHaveBeenCalledWith({ from: '2026-09-06', to: '2027-03-05' });
	});

	it('uses the browser zone before any settings exist', async () => {
		vi.spyOn(Intl.DateTimeFormat.prototype, 'resolvedOptions').mockReturnValue({
			timeZone: 'Pacific/Kiritimati'
		} as Intl.ResolvedDateTimeFormatOptions);
		const { deps, bootstrapCall } = setup(ok(bootstrap(0)));
		await refresh(deps);
		expect(bootstrapCall).toHaveBeenCalledWith({ from: '2026-09-06', to: '2027-03-05' });
	});

	it('leaves the cache and the badge alone when the server is off', async () => {
		await writeSnapshot(bootstrap(2, 'Ana Rojas'), '2026-11-03T12:00:00Z');
		const { deps, badge } = setup({ ok: false, error: { kind: 'offline' } });
		const result = await refresh(deps);
		expect(result).toEqual({ ok: false, error: { kind: 'offline' } });
		expect((await readAll())?.due).toHaveLength(2);
		expect((await readMeta()).lastSyncAt).toBe('2026-11-03T12:00:00Z');
		expect(badge.set).not.toHaveBeenCalled();
		expect(badge.clear).not.toHaveBeenCalled();
	});
});
