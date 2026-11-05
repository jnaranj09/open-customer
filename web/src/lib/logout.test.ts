import { describe, expect, it, vi } from 'vitest';
import type { Result } from './api/errors';
import { logOut } from './logout';

const ok: Result<void> = { ok: true, data: undefined };
const offline: Result<void> = { ok: false, error: { kind: 'offline' } };

function setup(answer: Result<void>, clearFails = false) {
	const calls: string[] = [];
	const deps = {
		logout: vi.fn(() => {
			calls.push('logout');
			return Promise.resolve(answer);
		}),
		clear: vi.fn(() => {
			calls.push('clear');
			return clearFails ? Promise.reject(new Error('blocked')) : Promise.resolve();
		}),
		toLogin: vi.fn(() => {
			calls.push('login');
			return Promise.resolve();
		})
	};
	return { deps, calls };
}

describe('logOut', () => {
	it('signs out, clears the cache and then shows the login screen', async () => {
		const { deps, calls } = setup(ok);
		await logOut(deps);
		expect(calls).toEqual(['logout', 'clear', 'login']);
	});

	it('keeps the cache and the screen when the server cannot be reached', async () => {
		const { deps, calls } = setup(offline);
		const result = await logOut(deps);
		expect([result, calls]).toEqual([offline, ['logout']]);
	});

	it('still shows the login screen when the cache cannot be cleared', async () => {
		const { deps, calls } = setup(ok, true);
		await logOut(deps);
		expect(calls).toEqual(['logout', 'clear', 'login']);
	});
});
