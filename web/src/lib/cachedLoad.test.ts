import { describe, expect, it } from 'vitest';
import type { ApiError, Result } from './api/errors';
import { showThenFetch } from './cachedLoad';

const answer = (r: Result<string>) => () => Promise.resolve(r);
const fail = (error: ApiError): Result<string> => ({ ok: false, error });

function run(cached: string | undefined, r: Result<string>, current = true) {
	const shown: (string | null)[] = [];
	const done = showThenFetch(
		cached,
		answer(r),
		() => current,
		(v) => {
			shown.push(v);
		}
	);
	return { shown, done };
}

describe('showThenFetch', () => {
	it('shows the cached copy first and the server copy after', async () => {
		const { shown, done } = run('old', { ok: true, data: 'new' });
		await done;
		expect(shown).toEqual(['old', 'new']);
	});

	it('shows nothing before the answer when there is no cached copy', async () => {
		const { shown, done } = run(undefined, { ok: true, data: 'new' });
		await done;
		expect(shown).toEqual([null, 'new']);
	});

	it('keeps the cached copy when the server is off', async () => {
		const { shown, done } = run('old', fail({ kind: 'offline' }));
		expect([await done, shown]).toEqual([null, ['old']]);
	});

	it('reports the server being off when there is no cached copy', async () => {
		const { done } = run(undefined, fail({ kind: 'offline' }));
		expect(await done).toEqual({ kind: 'offline' });
	});

	it('reports any other failure even with a cached copy', async () => {
		const { done } = run('old', fail({ kind: 'response_invalid', path: '/x' }));
		expect(await done).toEqual({ kind: 'response_invalid', path: '/x' });
	});

	it('says nothing when the screen moved on to another record', async () => {
		const { shown, done } = run('old', { ok: true, data: 'new' }, false);
		expect([await done, shown]).toEqual([undefined, ['old']]);
	});
});
