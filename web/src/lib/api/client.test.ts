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
	return { api, hooks, fetchFn };
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

describe('setup, icon and password', () => {
	const SETTINGS = {
		name: 'Estudio Luna',
		timezone: 'America/Costa_Rica',
		locale: 'es',
		currency: 'CRC',
		country_code: 'CR',
		accent_color: '#c76b85'
	};

	it('reads the country list of /setup/options', async () => {
		const { api } = setup(() => json({ countries: ['CR', 'ES'] }));
		expect(await api.setupOptions()).toEqual({ ok: true, data: { countries: ['CR', 'ES'] } });
	});

	it('posts the setup body and parses the new user', async () => {
		const { api } = setup(() => json(ME, 201));
		const result = await api.setup({
			token: 'tok',
			username: 'luna',
			password: 'a-long-password',
			settings: SETTINGS
		});
		expect(result).toEqual({ ok: true, data: ME });
	});

	it('maps a closed setup to its problem name', async () => {
		const { api } = setup(() => problem('setup-token-invalid', 410));
		const result = await api.setup({
			token: 'old',
			username: 'luna',
			password: 'a-long-password',
			settings: SETTINGS
		});
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', type: 'setup-token-invalid' }
		});
	});

	it('uploads both icon parts as multipart with the browser-made content type', async () => {
		const { api, fetchFn } = setup(
			() => new Response(null, { status: 204, headers: appHeaders() })
		);
		const result = await api.uploadIcon({
			big: new Blob(['a'], { type: 'image/png' }),
			small: new Blob(['b'], { type: 'image/png' })
		});
		const sent = fetchFn.mock.calls[0]?.[0] as Request;
		const form = await sent.formData();
		expect(result).toEqual({ ok: true, data: undefined });
		expect([...form.keys()]).toEqual(['icon512', 'icon180']);
	});

	it('changes the password with a 204', async () => {
		const { api } = setup(() => new Response(null, { status: 204, headers: appHeaders() }));
		expect(
			await api.changePassword({ current_password: 'old-password', new_password: 'new-password' })
		).toEqual({
			ok: true,
			data: undefined
		});
	});

	it('keeps a wrong current password as a validation problem on its field', async () => {
		const { api } = setup(() =>
			problem('validation', 422, {
				errors: [{ field: 'current_password', message: 'Clave incorrecta' }]
			})
		);
		const result = await api.changePassword({
			current_password: 'x',
			new_password: 'new-password'
		});
		expect(result).toMatchObject({
			ok: false,
			error: { fields: { current_password: 'Clave incorrecta' } }
		});
	});
});

describe('listAppointments', () => {
	it('asks for the range and returns the items', async () => {
		const { api, fetchFn } = setup(() => json({ items: [appointment] }));
		const result = await api.listAppointments({ from: '2026-11-02', to: '2026-11-08' });
		expect(result).toEqual({ ok: true, data: [appointment] });
		expect(fetchFn.mock.calls.map(([request]) => (request as Request).url)).toEqual([
			`${BASE}/appointments?from=2026-11-02&to=2026-11-08`
		]);
	});

	it('returns response_invalid for an item that fails its schema', async () => {
		const { api } = setup(() => json({ items: [{ ...appointment, status: 'done' }] }));
		const result = await api.listAppointments({ from: '2026-11-02', to: '2026-11-08' });
		expect(result).toEqual({
			ok: false,
			error: { kind: 'response_invalid', path: '/appointments' }
		});
	});
});

const logItem = {
	rule_id: USER_ID,
	appointment_id: USER_ID,
	anchor_at: null,
	action: 'sent',
	body: 'Hola Ana',
	phone_digits: '50688887777',
	acted_at: '2026-11-04T15:00:00Z',
	id: USER_ID,
	kind: 'come_back',
	client_id: USER_ID,
	client_name: 'Ana Rojas'
};

describe('recordReminder', () => {
	const entry = {
		rule_id: USER_ID,
		appointment_id: USER_ID,
		action: 'sent',
		acted_at: '2026-11-04T15:00:00Z'
	} as const;

	it('accepts the 201 row', async () => {
		const { api } = setup(() => json(logItem, 201));
		expect(await api.recordReminder(entry)).toEqual({ ok: true, data: logItem });
	});

	it('accepts the 200 row of an item that was already logged', async () => {
		const { api } = setup(() => json(logItem, 200));
		expect((await api.recordReminder(entry)).ok).toBe(true);
	});

	it('returns response_invalid for a row that fails its schema', async () => {
		const { api } = setup(() => json({ ...logItem, action: 'later' }, 201));
		expect(await api.recordReminder(entry)).toEqual({
			ok: false,
			error: { kind: 'response_invalid', path: '/reminders/log' }
		});
	});
});

describe('listReminderLog', () => {
	it('asks for the day and returns the items', async () => {
		const { api, fetchFn } = setup(() => json({ items: [logItem] }));
		const result = await api.listReminderLog({ date: '2026-11-04' });
		expect(result).toEqual({ ok: true, data: [logItem] });
		expect(fetchFn.mock.calls.map(([request]) => (request as Request).url)).toEqual([
			`${BASE}/reminders/log?date=2026-11-04`
		]);
	});

	it('asks for one client', async () => {
		const { api, fetchFn } = setup(() => json({ items: [logItem] }));
		await api.listReminderLog({ client_id: USER_ID });
		expect(fetchFn.mock.calls.map(([request]) => (request as Request).url)).toEqual([
			`${BASE}/reminders/log?client_id=${USER_ID}`
		]);
	});
});

describe('updateAppointment', () => {
	it('sends the patch to the appointment and returns it', async () => {
		const { api, fetchFn } = setup(() => json(appointment));
		const result = await api.updateAppointment(USER_ID, { service_ids: ['s1'] });
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		expect([result.ok, request.method, request.url, await request.text()]).toEqual([
			true,
			'PATCH',
			`${BASE}/appointments/${USER_ID}`,
			'{"service_ids":["s1"]}'
		]);
	});

	it('types a 409 as overlap', async () => {
		const { api } = setup(() => problem('overlap', 409, { appointments: [appointment] }));
		const result = await api.updateAppointment(USER_ID, { service_ids: ['s1'] });
		expect(result).toEqual({ ok: false, error: { kind: 'overlap', overlapping: [appointment] } });
	});
});

describe('getAppointment', () => {
	it('reads the appointment by id', async () => {
		const { api, fetchFn } = setup(() => json(appointment));
		const result = await api.getAppointment(USER_ID);
		expect([result, (fetchFn.mock.calls[0]?.[0] as Request).url]).toEqual([
			{ ok: true, data: appointment },
			`${BASE}/appointments/${USER_ID}`
		]);
	});

	it('types a 404 as a problem', async () => {
		const { api } = setup(() => problem('not-found', 404));
		const result = await api.getAppointment(USER_ID);
		expect(result).toMatchObject({ ok: false, error: { kind: 'problem', type: 'not-found' } });
	});
});

const media = {
	id: USER_ID,
	client_id: USER_ID,
	appointment_id: USER_ID,
	width: 2048,
	height: 1536,
	bytes: 120000,
	created_at: '2026-11-04T15:00:00Z',
	archived: false
};

describe('createAppointment', () => {
	it('posts the body and returns the appointment', async () => {
		const { api, fetchFn } = setup(() => json(appointment, 201));
		const body = { client_id: USER_ID, starts_at: '2026-11-04T16:00:00Z', service_ids: ['s1'] };
		const result = await api.createAppointment(body);
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		expect([result.ok, request.method, request.url, await request.text()]).toEqual([
			true,
			'POST',
			`${BASE}/appointments`,
			JSON.stringify(body)
		]);
	});

	it('types a 409 as overlap with the appointments in the way', async () => {
		const { api } = setup(() => problem('overlap', 409, { appointments: [appointment] }));
		const result = await api.createAppointment({
			client_id: USER_ID,
			starts_at: appointment.starts_at
		});
		expect(result).toEqual({ ok: false, error: { kind: 'overlap', overlapping: [appointment] } });
	});
});

describe('setAppointmentStatus', () => {
	it('posts the new status to the appointment', async () => {
		const { api, fetchFn } = setup(() => json(appointment));
		const result = await api.setAppointmentStatus(USER_ID, { status: 'confirmed' });
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		expect([result.ok, request.method, request.url, await request.text()]).toEqual([
			true,
			'POST',
			`${BASE}/appointments/${USER_ID}/status`,
			'{"status":"confirmed"}'
		]);
	});

	it('types a 409 invalid-transition as a problem', async () => {
		const { api } = setup(() => problem('invalid-transition', 409));
		const result = await api.setAppointmentStatus(USER_ID, { status: 'confirmed' });
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', type: 'invalid-transition' }
		});
	});
});

describe('createClient', () => {
	const client = {
		id: USER_ID,
		name: 'Ana Rojas',
		phone_e164: null,
		phone_digits: null,
		notes: null,
		attrs: {},
		archived: false,
		created_at: '2026-11-01T10:00:00Z',
		updated_at: '2026-11-01T10:00:00Z'
	};

	it('posts the client and returns it', async () => {
		const { api, fetchFn } = setup(() => json(client, 201));
		const result = await api.createClient({ name: 'Ana Rojas' });
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		expect([result, request.method, request.url]).toEqual([
			{ ok: true, data: client },
			'POST',
			`${BASE}/clients`
		]);
	});

	it('hands back the clients that share the phone on a duplicate-phone 409', async () => {
		const { api } = setup(() => problem('duplicate-phone', 409, { clients: [client] }));
		const result = await api.createClient({ name: 'Ana', phone: '8888-7777' });
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', type: 'duplicate-phone', problem: { clients: [client] } }
		});
	});
});

describe('the client reads and changes', () => {
	const client = {
		id: USER_ID,
		name: 'Ana Rojas',
		phone_e164: null,
		phone_digits: null,
		notes: null,
		attrs: {},
		archived: false,
		created_at: '2026-11-01T10:00:00Z',
		updated_at: '2026-11-01T10:00:00Z'
	};

	it('listClients asks for the archived ones and returns the items', async () => {
		const { api, fetchFn } = setup(() => json({ items: [client] }));
		const result = await api.listClients({ archived: true });
		expect([result, (fetchFn.mock.calls[0]?.[0] as Request).url]).toEqual([
			{ ok: true, data: [client] },
			`${BASE}/clients?archived=true`
		]);
	});

	it('getClient asks for the client', async () => {
		const { api, fetchFn } = setup(() => json(client));
		const result = await api.getClient(USER_ID);
		expect([result, (fetchFn.mock.calls[0]?.[0] as Request).url]).toEqual([
			{ ok: true, data: client },
			`${BASE}/clients/${USER_ID}`
		]);
	});

	it('updateClient patches only the fields sent', async () => {
		const { api, fetchFn } = setup(() => json(client));
		const result = await api.updateClient(USER_ID, { phone: null });
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		expect([result.ok, request.method, request.url, await request.text()]).toEqual([
			true,
			'PATCH',
			`${BASE}/clients/${USER_ID}`,
			'{"phone":null}'
		]);
	});

	it('updateClient hands back the clients that share the phone on a 409', async () => {
		const { api } = setup(() => problem('duplicate-phone', 409, { clients: [client] }));
		const result = await api.updateClient(USER_ID, { phone: '8888-7777' });
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', type: 'duplicate-phone', problem: { clients: [client] } }
		});
	});

	it('listClientAppointments returns the items of that client', async () => {
		const { api, fetchFn } = setup(() => json({ items: [appointment] }));
		const result = await api.listClientAppointments(USER_ID);
		expect([result, (fetchFn.mock.calls[0]?.[0] as Request).url]).toEqual([
			{ ok: true, data: [appointment] },
			`${BASE}/clients/${USER_ID}/appointments`
		]);
	});
});

describe('listClientMedia', () => {
	it('asks for the client and returns the photos', async () => {
		const { api, fetchFn } = setup(() => json({ items: [media] }));
		const result = await api.listClientMedia(USER_ID);
		expect([result, (fetchFn.mock.calls[0]?.[0] as Request).url]).toEqual([
			{ ok: true, data: [media] },
			`${BASE}/clients/${USER_ID}/media`
		]);
	});

	it('returns response_invalid for a photo that fails its schema', async () => {
		const { api } = setup(() => json({ items: [{ ...media, width: 'wide' }] }));
		const result = await api.listClientMedia(USER_ID);
		expect(result).toEqual({
			ok: false,
			error: { kind: 'response_invalid', path: '/clients/{id}/media' }
		});
	});
});

describe('uploadMedia', () => {
	it('sends the two parts and the appointment as multipart', async () => {
		const { api, fetchFn } = setup(() => json(media, 201));
		const full = new Blob(['f'], { type: 'image/jpeg' });
		const thumb = new Blob(['t'], { type: 'image/jpeg' });
		const result = await api.uploadMedia({ full, thumb, appointment_id: USER_ID });
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		const form = await request.formData();
		expect([
			result.ok,
			request.method,
			request.url,
			[...form.keys()].sort(),
			form.get('appointment_id')
		]).toEqual([true, 'POST', `${BASE}/media`, ['appointment_id', 'full', 'thumb'], USER_ID]);
	});

	it('sends the client when the photo belongs to no appointment', async () => {
		const { api, fetchFn } = setup(() => json(media, 201));
		const blob = new Blob(['f'], { type: 'image/jpeg' });
		await api.uploadMedia({ full: blob, thumb: blob, client_id: USER_ID });
		const form = await (fetchFn.mock.calls[0]?.[0] as Request).formData();
		expect([[...form.keys()].sort(), form.get('client_id')]).toEqual([
			['client_id', 'full', 'thumb'],
			USER_ID
		]);
	});

	it('types a 415 as a problem', async () => {
		const { api } = setup(() => problem('unsupported-media-type', 415));
		const blob = new Blob(['f']);
		const result = await api.uploadMedia({ full: blob, thumb: blob, appointment_id: USER_ID });
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', type: 'unsupported-media-type' }
		});
	});
});

describe('archiveMedia', () => {
	it('patches archived to true', async () => {
		const { api, fetchFn } = setup(() => json({ ...media, archived: true }));
		const result = await api.archiveMedia(USER_ID);
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		expect([result.ok, request.method, request.url, await request.text()]).toEqual([
			true,
			'PATCH',
			`${BASE}/media/${USER_ID}`,
			'{"archived":true}'
		]);
	});
});

describe('mediaThumbUrl', () => {
	it('names the thumbnail of a photo', () => {
		const { api } = setup(() => json({}));
		expect(api.mediaThumbUrl(USER_ID)).toBe(`${BASE}/media/${USER_ID}/thumb`);
	});
});

describe('the template calls', () => {
	const template = {
		id: USER_ID,
		name: 'Recordatorio',
		body: 'Hola {{client.name}}',
		archived: false
	};
	const sent = async (fetchFn: ReturnType<typeof setup>['fetchFn']): Promise<unknown[]> => {
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		return [request.method, request.url, (await request.json()) as unknown];
	};

	it('createTemplate posts the name and body and returns the template', async () => {
		const { api, fetchFn } = setup(() => json(template, 201));
		const result = await api.createTemplate({ name: template.name, body: template.body });
		expect([result, await sent(fetchFn)]).toEqual([
			{ ok: true, data: template },
			['POST', `${BASE}/templates`, { name: template.name, body: template.body }]
		]);
	});

	it('updateTemplate patches only what is sent', async () => {
		const { api, fetchFn } = setup(() => json({ ...template, archived: true }));
		const result = await api.updateTemplate(USER_ID, { archived: true });
		expect([result, await sent(fetchFn)]).toEqual([
			{ ok: true, data: { ...template, archived: true } },
			['PATCH', `${BASE}/templates/${USER_ID}`, { archived: true }]
		]);
	});

	it('previewTemplate posts the body and the client and returns the text', async () => {
		const answer = { text: 'Hola Ana', unknown_vars: [] };
		const { api, fetchFn } = setup(() => json(answer));
		const result = await api.previewTemplate({ body: 'Hola', client_id: USER_ID });
		expect([result, await sent(fetchFn)]).toEqual([
			{ ok: true, data: answer },
			['POST', `${BASE}/templates/preview`, { body: 'Hola', client_id: USER_ID }]
		]);
	});

	it('previewTemplate reports an answer without text as invalid', async () => {
		const { api, hooks } = setup(() => json({ unknown_vars: [] }));
		const result = await api.previewTemplate({ body: 'Hola' });
		expect([result.ok, hooks.reportInvalid.mock.calls]).toEqual([false, [['/templates/preview']]]);
	});

	it('createTemplate hands back the server messages of a 422 by field', async () => {
		const { api } = setup(() =>
			problem('validation', 422, { errors: [{ field: 'body', message: 'Hay un {{ sin cerrar' }] })
		);
		const result = await api.createTemplate({ name: 'x', body: '{{' });
		expect(result).toMatchObject({
			ok: false,
			error: { kind: 'problem', fields: { body: 'Hay un {{ sin cerrar' } }
		});
	});
});

describe('the settings calls', () => {
	const PINK = ['#', 'c76b85'].join('');
	const OPTION_ID = '7a1c9e20-2b3d-4f5a-8b6c-0d9e8f7a6b5c';
	const settings = {
		name: 'Estudio Ana',
		timezone: 'America/Costa_Rica',
		locale: 'es',
		currency: 'CRC',
		country_code: 'CR',
		accent_color: PINK,
		push_hour: 9,
		has_custom_icon: false
	};
	const service = {
		id: USER_ID,
		name: 'Acrílico',
		duration_min: 90,
		price_cents: null,
		color: PINK,
		position: 0,
		archived: false
	};
	const option = {
		id: OPTION_ID,
		attribute_id: USER_ID,
		parent_id: null,
		label: 'Almendra',
		position: 0,
		archived: false
	};
	const attribute = {
		id: USER_ID,
		key: 'forma',
		label: 'Forma',
		scope: 'appointment',
		type: 'select',
		required: false,
		position: 0,
		archived: false,
		options: [option]
	};

	const sent = async (fetchFn: ReturnType<typeof setup>['fetchFn']): Promise<unknown[]> => {
		const request = fetchFn.mock.calls[0]?.[0] as Request;
		return [request.method, request.url, (await request.json()) as unknown];
	};

	it('updateSettings patches only what is sent', async () => {
		const { api, fetchFn } = setup(() => json(settings));
		const result = await api.updateSettings({ push_hour: 9 });
		expect([result, await sent(fetchFn)]).toEqual([
			{ ok: true, data: settings },
			['PATCH', `${BASE}/settings`, { push_hour: 9 }]
		]);
	});

	it('createService posts the service and returns it', async () => {
		const { api, fetchFn } = setup(() => json(service, 201));
		const body = { name: 'Acrílico', duration_min: 90, price_cents: null, color: PINK };
		const result = await api.createService(body);
		expect([result, await sent(fetchFn)]).toEqual([
			{ ok: true, data: service },
			['POST', `${BASE}/services`, body]
		]);
	});

	it('updateService patches the service by id', async () => {
		const { api, fetchFn } = setup(() => json({ ...service, archived: true }));
		const result = await api.updateService(USER_ID, { archived: true });
		expect([result.ok, await sent(fetchFn)]).toEqual([
			true,
			['PATCH', `${BASE}/services/${USER_ID}`, { archived: true }]
		]);
	});

	it('createAttribute posts the label, scope and type', async () => {
		const { api, fetchFn } = setup(() => json(attribute, 201));
		const body = { label: 'Forma', scope: 'appointment', type: 'select', required: false } as const;
		const result = await api.createAttribute(body);
		expect([result, await sent(fetchFn)]).toEqual([
			{ ok: true, data: attribute },
			['POST', `${BASE}/attributes`, body]
		]);
	});

	it('updateAttribute patches the attribute by id', async () => {
		const { api, fetchFn } = setup(() => json(attribute));
		const result = await api.updateAttribute(USER_ID, { label: 'Forma' });
		expect([result.ok, await sent(fetchFn)]).toEqual([
			true,
			['PATCH', `${BASE}/attributes/${USER_ID}`, { label: 'Forma' }]
		]);
	});

	it('createOption posts under the attribute, with the parent when given', async () => {
		const { api, fetchFn } = setup(() => json({ ...option, parent_id: USER_ID }, 201));
		const result = await api.createOption(USER_ID, { label: 'Corta', parent_id: USER_ID });
		expect([result.ok, await sent(fetchFn)]).toEqual([
			true,
			['POST', `${BASE}/attributes/${USER_ID}/options`, { label: 'Corta', parent_id: USER_ID }]
		]);
	});

	it('updateOption patches the option of the attribute', async () => {
		const { api, fetchFn } = setup(() => json({ ...option, position: 2 }));
		const result = await api.updateOption(USER_ID, OPTION_ID, { position: 2 });
		expect([result.ok, await sent(fetchFn)]).toEqual([
			true,
			['PATCH', `${BASE}/attributes/${USER_ID}/options/${OPTION_ID}`, { position: 2 }]
		]);
	});

	it('updateOption reports an answer without an id as invalid', async () => {
		const { api, hooks } = setup(() => json({ label: 'x' }));
		const result = await api.updateOption(USER_ID, OPTION_ID, { label: 'x' });
		expect([result.ok, hooks.reportInvalid.mock.calls]).toEqual([
			false,
			[['/attributes/{id}/options/{optionId}']]
		]);
	});
});
