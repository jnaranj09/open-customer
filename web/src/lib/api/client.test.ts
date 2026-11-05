import { describe, expect, it, vi } from 'vitest';
import { createApi } from './client';
import { errorText } from './errors';

const BASE = 'http://app.test/api/v1';
const VERSION = 'v1';
const USER_ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const ME = { id: USER_ID, username: 'ana', must_change_password: false };
const PREFIX = 'urn:open-customer:problem:';

const appHeaders = (version = VERSION): Record<string, string> => ({ 'X-App-Version': version });

const json = (body: unknown, status = 200, headers = appHeaders()): Response =>
	new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json', ...headers }
	});

const problem = (name: string, status: number, extra: object = {}): Response =>
	new Response(JSON.stringify({ type: PREFIX + name, title: 'x', status, ...extra }), {
		status,
		headers: { 'Content-Type': 'application/problem+json', ...appHeaders() }
	});

function setup(answer: () => Response | Promise<Response>) {
	const fetchFn = vi.fn<typeof fetch>(() => Promise.resolve(answer()));
	const hooks = {
		onUnauthorized: vi.fn(() => Promise.resolve()),
		onNewVersion: vi.fn(),
		reportInvalid: vi.fn()
	};
	const api = createApi({ fetch: fetchFn, version: VERSION, baseUrl: BASE, ...hooks });
	return { api, hooks };
}

const appointment = {
	id: USER_ID,
	client_id: USER_ID,
	client_name: 'Ana Rojas',
	starts_at: '2026-11-04T10:00:00-06:00',
	ends_at: '2026-11-04T11:00:00-06:00',
	status: 'scheduled',
	done: false,
	total_cents: 0,
	paid_cents: null,
	attrs: {},
	notes: null,
	services: [],
	created_at: '2026-11-01T10:00:00Z',
	updated_at: '2026-11-01T10:00:00Z'
};

describe('offline detection', () => {
	it('treats a 530 HTML answer without the version header as offline', async () => {
		const { api } = setup(
			() =>
				new Response('<html>error 1033</html>', {
					status: 530,
					headers: { 'Content-Type': 'text/html' }
				})
		);
		expect(await api.me()).toEqual({ ok: false, error: { kind: 'offline' } });
	});

	it('treats a network error as offline', async () => {
		const { api } = setup(() => Promise.reject(new TypeError('fetch failed')));
		expect(await api.me()).toEqual({ ok: false, error: { kind: 'offline' } });
	});
});

describe('answers from the app', () => {
	it('treats a 500 problem with the header as a problem, not offline', async () => {
		const { api } = setup(() => problem('internal', 500));
		const result = await api.me();
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', status: 500, type: 'internal' }
		});
	});

	it('treats a 204 with the header as success', async () => {
		const { api } = setup(() => new Response(null, { status: 204, headers: appHeaders() }));
		expect(await api.logout()).toEqual({ ok: true, data: undefined });
	});

	it('returns the parsed body of a 200', async () => {
		const { api } = setup(() => json(ME));
		expect(await api.me()).toEqual({ ok: true, data: ME });
	});

	it('types a 409 overlap with the overlapping appointments', async () => {
		const { api } = setup(() => problem('overlap', 409, { appointments: [appointment] }));
		const result = await api.me();
		expect(result).toEqual({ ok: false, error: { kind: 'overlap', overlapping: [appointment] } });
	});

	it('collects the field messages of a validation problem', async () => {
		const { api } = setup(() =>
			problem('validation', 422, { errors: [{ field: 'phone', message: 'Teléfono no válido' }] })
		);
		const result = await api.me();
		expect(result).toMatchObject({
			ok: false,
			error: { type: 'validation', fields: { phone: 'Teléfono no válido' } }
		});
	});
});

describe('version header', () => {
	it('calls onNewVersion when the header differs from the app version', async () => {
		const { api, hooks } = setup(() => json(ME, 200, appHeaders('v2')));
		await api.me();
		expect(hooks.onNewVersion).toHaveBeenCalledTimes(1);
	});

	it('does not call onNewVersion when the header matches', async () => {
		const { api, hooks } = setup(() => json(ME));
		await api.me();
		expect(hooks.onNewVersion).not.toHaveBeenCalled();
	});
});

describe('response schema', () => {
	it('returns response_invalid for a 200 body that fails its schema and reports the path only', async () => {
		const { api, hooks } = setup(() =>
			json({ ...ME, username: 'Ána López secret', id: 'not-a-uuid' })
		);
		const result = await api.me();
		expect(result).toEqual({ ok: false, error: { kind: 'response_invalid', path: '/me' } });
		expect(hooks.reportInvalid).toHaveBeenCalledWith('/me');
	});

	it('returns response_invalid for an error body that is not a problem', async () => {
		const { api, hooks } = setup(() => json({ nope: true }, 500));
		const result = await api.me();
		expect(result).toEqual({ ok: false, error: { kind: 'response_invalid', path: '/me' } });
		expect(hooks.reportInvalid).toHaveBeenCalledWith('/me');
	});
});

describe('unauthorized', () => {
	it('returns unauthorized when onUnauthorized throws', async () => {
		vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const { api, hooks } = setup(() => problem('unauthorized', 401));
		hooks.onUnauthorized.mockImplementation(() => Promise.reject(new Error('hook failed')));
		expect(await api.me()).toEqual({ ok: false, error: { kind: 'unauthorized' } });
	});

	it('calls onUnauthorized on a 401 unauthorized and returns unauthorized', async () => {
		const { api, hooks } = setup(() => problem('unauthorized', 401));
		const result = await api.me();
		expect(result).toEqual({ ok: false, error: { kind: 'unauthorized' } });
		expect(hooks.onUnauthorized).toHaveBeenCalledTimes(1);
	});

	it('does not call onUnauthorized for invalid-credentials on login', async () => {
		const { api, hooks } = setup(() => problem('invalid-credentials', 401));
		const result = await api.login({ username: 'ana', password: 'wrong-password' });
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', type: 'invalid-credentials' }
		});
		expect(hooks.onUnauthorized).not.toHaveBeenCalled();
	});
});

describe('errorText', () => {
	it('shows the field messages of a validation problem', async () => {
		const { api } = setup(() =>
			problem('validation', 422, { errors: [{ field: 'phone', message: 'Teléfono no válido' }] })
		);
		const result = await api.me();
		expect(result.ok ? '' : errorText(result.error)).toBe('Teléfono no válido');
	});

	it('maps a problem type to its Spanish text, not the server detail', async () => {
		const { api } = setup(() => problem('rate-limited', 429, { detail: 'secret detail' }));
		const result = await api.me();
		expect(result.ok ? '' : errorText(result.error)).toBe('Demasiados intentos. Espera un momento');
	});

	it('maps offline to its text', () => {
		expect(errorText({ kind: 'offline' })).toBe('Sin conexión con el sistema');
	});
});

describe('push', () => {
	it('reads the VAPID key', async () => {
		const { api } = setup(() => json({ public_key: 'AQIDBA' }));
		expect(await api.pushKey()).toEqual({ ok: true, data: { public_key: 'AQIDBA' } });
	});

	it('stores a subscription and reads its id', async () => {
		const { api } = setup(() => json({ id: USER_ID }, 201));
		const result = await api.subscribePush({
			endpoint: 'https://push.example.test/abc',
			expirationTime: null,
			keys: { p256dh: 'p', auth: 'a' }
		});
		expect(result).toEqual({ ok: true, data: { id: USER_ID } });
	});

	it('reports a subscription answer without an id as response_invalid', async () => {
		const { api } = setup(() => json({}, 201));
		const result = await api.subscribePush({ endpoint: 'x', keys: { p256dh: 'p', auth: 'a' } });
		expect(result).toEqual({
			ok: false,
			error: { kind: 'response_invalid', path: '/push/subscriptions' }
		});
	});
});
