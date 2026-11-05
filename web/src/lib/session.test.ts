import { describe, expect, it, vi } from 'vitest';
import type { Api } from './api/client';
import type { ApiError, Result } from './api/errors';
import { resolveSession } from './session';

const ME = {
	id: '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f',
	username: 'ana',
	must_change_password: false
};
const CACHED = { ...ME, username: 'cached' };

const fail = (error: ApiError): Result<never> => ({ ok: false, error });
const problem: ApiError = { kind: 'response_invalid', path: '/me' };

function api(status: Result<{ needs_setup: boolean }>, me?: Result<typeof ME>) {
	const meCall = vi.fn<Api['me']>(() => Promise.resolve(me ?? { ok: true, data: ME }));
	const setupStatus = vi.fn<Api['setupStatus']>(() => Promise.resolve(status));
	return { api: { me: meCall, setupStatus }, meCall };
}

describe('resolveSession', () => {
	it('is online with the user when the server knows her', async () => {
		const { api: a } = api({ ok: true, data: { needs_setup: false } });
		expect(await resolveSession(a, null)).toEqual({ kind: 'online', setupNeeded: false, me: ME });
	});

	it('is online with setup needed, without asking for her', async () => {
		const { api: a, meCall } = api({ ok: true, data: { needs_setup: true } });
		expect(await resolveSession(a, null)).toEqual({ kind: 'online', setupNeeded: true, me: null });
		expect(meCall).not.toHaveBeenCalled();
	});

	it('is online without a user on an unauthorized answer', async () => {
		const { api: a } = api(
			{ ok: true, data: { needs_setup: false } },
			fail({ kind: 'unauthorized' })
		);
		expect(await resolveSession(a, CACHED)).toEqual({
			kind: 'online',
			setupNeeded: false,
			me: null
		});
	});

	it('is offline with the cached user when the server does not answer', async () => {
		const { api: a } = api(fail({ kind: 'offline' }));
		expect(await resolveSession(a, CACHED)).toEqual({ kind: 'offline', cachedMe: CACHED });
	});

	it('is offline when only /me does not answer', async () => {
		const { api: a } = api({ ok: true, data: { needs_setup: false } }, fail({ kind: 'offline' }));
		expect(await resolveSession(a, null)).toEqual({ kind: 'offline', cachedMe: null });
	});

	it('treats an answer it cannot read as offline: it never redirects on a guess', async () => {
		const { api: a } = api({ ok: true, data: { needs_setup: false } }, fail(problem));
		expect(await resolveSession(a, CACHED)).toEqual({ kind: 'offline', cachedMe: CACHED });
	});
});
