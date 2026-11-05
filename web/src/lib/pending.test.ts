import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Api } from './api/client';
import type { Result } from './api/errors';
import type { DueItem, LogItem } from './api/schemas';
import { clearCache, readMeta } from './cache/db';
import type { Badge } from './cache/refresh';
import { createCache } from './cache/store.svelte';
import { confirmSent, notYet, pendingToAsk, skipItem, startSend } from './pending';

const NOW = new Date('2026-11-04T15:00:00Z');
const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';

function item(id: string): DueItem & { wa_link: string } {
	return {
		id,
		rule_id: ID,
		appointment_id: ID,
		anchor_at: '2026-11-05T21:00:00Z',
		kind: 'before_appointment',
		client_id: ID,
		client_name: 'Ana Rojas',
		due_date: '2026-11-04',
		late: false,
		text: 'Hola Ana, te espero mañana',
		phone_digits: '50688887777',
		wa_link: 'https://wa.me/50688887777?text=Hola',
		mode: 'message',
		prompt: null
	};
}

const logged = { ok: true, data: {} as LogItem } satisfies Result<LogItem>;

function setup(answer: Result<LogItem> = logged) {
	const cache = createCache({
		api: {
			bootstrap: () => Promise.resolve({ ok: false, error: { kind: 'offline' } })
		} as unknown as Api,
		now: () => NOW,
		badge: { set: () => Promise.resolve(), clear: () => Promise.resolve() }
	});
	const recordReminder = vi.fn<Api['recordReminder']>(() => Promise.resolve(answer));
	const badge = {
		set: vi.fn<Badge['set']>(() => Promise.resolve()),
		clear: vi.fn<Badge['clear']>(() => Promise.resolve())
	};
	const deps = { api: { recordReminder }, store: cache, badge, now: () => NOW };
	return { cache, deps, recordReminder, badge };
}

const PENDING = { item_id: 'a', client_name: 'Ana Rojas', started_at: NOW.toISOString() };

beforeEach(async () => {
	await clearCache();
});

describe('startSend', () => {
	it('stores the confirmation before opening the link', async () => {
		const { deps, cache } = setup();
		const storedAtOpen: unknown[] = [];
		const open = (): void => {
			storedAtOpen.push(cache.meta.pending);
		};
		await startSend(item('a'), open, deps);
		expect(storedAtOpen).toEqual([PENDING]);
	});

	it('opens the WhatsApp link of the item', async () => {
		const { deps } = setup();
		const open = vi.fn<(url: string) => void>();
		await startSend(item('a'), open, deps);
		expect(open).toHaveBeenCalledWith('https://wa.me/50688887777?text=Hola');
	});

	it('does not open the link when the confirmation cannot be stored', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const { deps } = setup();
		const open = vi.fn();
		const failing = {
			...deps,
			store: { ...deps.store, setPending: () => Promise.reject(new Error('x')) }
		};
		const result = await startSend(item('a'), open, failing);
		expect([result, open.mock.calls.length]).toEqual([
			{ ok: false, error: { kind: 'cache_write_failed' } },
			0
		]);
	});

	it('the confirmation survives a reload', async () => {
		const { deps } = setup();
		await startSend(item('a'), () => undefined, deps);
		const reloaded = setup().cache;
		await reloaded.load();
		expect(reloaded.meta.pending).toEqual(PENDING);
	});
});

describe('pendingToAsk', () => {
	it('is none without a confirmation', () => {
		expect(pendingToAsk(null, [item('a')])).toBe('none');
	});

	it('asks while the item is in the due list', () => {
		expect(pendingToAsk(PENDING, [item('a')])).toBe('ask');
	});

	it('clears a confirmation whose item is gone', () => {
		expect(pendingToAsk(PENDING, [item('b')])).toBe('clear');
	});
});

describe('confirmSent', () => {
	it('posts the log entry with the exact text that was sent', async () => {
		const { deps, recordReminder } = setup();
		await confirmSent(item('a'), { ...deps, due: [item('a')] });
		expect(recordReminder).toHaveBeenCalledWith({
			rule_id: ID,
			appointment_id: ID,
			anchor_at: '2026-11-05T21:00:00Z',
			action: 'sent',
			body: 'Hola Ana, te espero mañana',
			phone_digits: '50688887777',
			acted_at: '2026-11-04T15:00:00.000Z'
		});
	});

	it('clears the confirmation on success', async () => {
		const { deps, cache } = setup();
		await startSend(item('a'), () => undefined, deps);
		const result = await confirmSent(item('a'), { ...deps, due: [item('a')] });
		expect([result.ok, cache.meta.pending, (await readMeta()).pending]).toEqual([true, null, null]);
	});

	it('keeps the confirmation when the POST fails', async () => {
		const { deps, cache } = setup({ ok: false, error: { kind: 'offline' } });
		await startSend(item('a'), () => undefined, deps);
		const result = await confirmSent(item('a'), { ...deps, due: [item('a')] });
		expect([result, cache.meta.pending]).toEqual([
			{ ok: false, error: { kind: 'offline' } },
			PENDING
		]);
	});

	it('leaves the badge alone when the POST fails', async () => {
		const { deps, badge } = setup({ ok: false, error: { kind: 'offline' } });
		await confirmSent(item('a'), { ...deps, due: [item('a')] });
		expect([badge.set.mock.calls.length, badge.clear.mock.calls.length]).toEqual([0, 0]);
	});

	it('sets the badge to the items that are left', async () => {
		const { deps, badge } = setup();
		await confirmSent(item('a'), { ...deps, due: [item('a'), item('b'), item('c')] });
		expect(badge.set).toHaveBeenCalledWith(2);
	});

	it('after the last log POST the badge is cleared', async () => {
		const { deps, badge } = setup();
		await confirmSent(item('a'), { ...deps, due: [item('a')] });
		expect(badge.clear).toHaveBeenCalledOnce();
	});
});

describe('notYet', () => {
	it('clears the confirmation', async () => {
		const { deps, cache } = setup();
		await startSend(item('a'), () => undefined, deps);
		await notYet(cache);
		expect([cache.meta.pending, (await readMeta()).pending]).toEqual([null, null]);
	});
});

describe('logged items', () => {
	it('confirmSent hides the item from the cached list once the server logged it', async () => {
		const { deps, cache } = setup();
		await confirmSent(item('a'), { ...deps, due: [item('a')] });
		expect(cache.logged).toEqual(['a']);
	});

	it('confirmSent does not hide the item when the POST fails', async () => {
		const { deps, cache } = setup({ ok: false, error: { kind: 'offline' } });
		await confirmSent(item('a'), { ...deps, due: [item('a')] });
		expect(cache.logged).toEqual([]);
	});

	it('skipItem hides the item once the server logged it', async () => {
		const { deps, cache } = setup();
		await skipItem(item('a'), { ...deps, due: [item('a')] });
		expect(cache.logged).toEqual(['a']);
	});

	it('skipItem does not hide the item when the POST fails', async () => {
		const { deps, cache } = setup({ ok: false, error: { kind: 'offline' } });
		await skipItem(item('a'), { ...deps, due: [item('a')] });
		expect(cache.logged).toEqual([]);
	});
});

describe('skipItem', () => {
	it('posts a skipped entry without a body', async () => {
		const { deps, recordReminder } = setup();
		await skipItem(item('a'), { ...deps, due: [item('a')] });
		expect(recordReminder).toHaveBeenCalledWith({
			rule_id: ID,
			appointment_id: ID,
			anchor_at: '2026-11-05T21:00:00Z',
			action: 'skipped',
			acted_at: '2026-11-04T15:00:00.000Z'
		});
	});

	it('sets the badge to the items that are left', async () => {
		const { deps, badge } = setup();
		await skipItem(item('a'), { ...deps, due: [item('a'), item('b')] });
		expect(badge.set).toHaveBeenCalledWith(1);
	});

	it('returns the error and leaves the badge when the POST fails', async () => {
		const { deps, badge } = setup({ ok: false, error: { kind: 'offline' } });
		const result = await skipItem(item('a'), { ...deps, due: [item('a')] });
		expect([result, badge.set.mock.calls.length + badge.clear.mock.calls.length]).toEqual([
			{ ok: false, error: { kind: 'offline' } },
			0
		]);
	});
});
