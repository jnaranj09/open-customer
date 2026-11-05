import { describe, expect, it } from 'vitest';
import type { ApiError } from './api/errors';
import type { Appointment, Attribute, Client } from './api/schemas';
import {
	clientCreateBody,
	clientFormErrors,
	clientFormFrom,
	clientPatchBody,
	historySplit,
	restoreClientForm,
	sharingNames,
	visibleClients,
	type ClientForm
} from './clients';

const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const ZONE = 'America/Costa_Rica';
const WINDOW = { from: '2026-09-05', to: '2027-03-04' };

const appt = (starts_at: string): Appointment => ({
	id: ID,
	client_id: ID,
	client_name: 'Ana Rojas',
	starts_at,
	ends_at: starts_at,
	status: 'scheduled',
	done: false,
	total_cents: 0,
	paid_cents: null,
	attrs: {},
	notes: null,
	services: [],
	created_at: starts_at,
	updated_at: starts_at
});

const client = (name: string, over: Partial<Client> = {}): Client => ({
	id: ID,
	name,
	phone_e164: null,
	phone_digits: null,
	notes: null,
	attrs: {},
	archived: false,
	created_at: '2026-11-01T10:00:00Z',
	updated_at: '2026-11-01T10:00:00Z',
	...over
});

const attribute = (key: string, type: Attribute['type']): Attribute => ({
	id: ID,
	key,
	label: key,
	scope: 'client',
	type,
	required: false,
	position: 0,
	archived: false,
	options: []
});

const DEFS = [attribute('alergias', 'text'), attribute('hijos', 'number')];

const form = (over: Partial<ClientForm> = {}): ClientForm => ({
	name: 'Ana Rojas',
	phone: '',
	notes: '',
	attrs: {},
	...over
});

describe('historySplit', () => {
	it('puts appointments outside the window in "load more"', () => {
		const before = appt('2026-01-10T16:00:00Z');
		const after = appt('2027-06-10T16:00:00Z');
		const inside = appt('2026-11-04T16:00:00Z');
		expect(historySplit([before, inside, after], WINDOW, ZONE)).toEqual({
			cached: [inside],
			needsServer: [before, after]
		});
	});

	it('keeps the first and the last day of the window in the cache', () => {
		const first = appt('2026-09-05T16:00:00Z');
		const last = appt('2027-03-04T16:00:00Z');
		expect(historySplit([first, last], WINDOW, ZONE).cached).toEqual([first, last]);
	});

	it('reads the day in the business zone, not in UTC', () => {
		// 03:00 UTC on the 5th is 21:00 on the 4th in Costa Rica: a day before the window.
		const late = appt('2026-09-05T03:00:00Z');
		expect(historySplit([late], WINDOW, ZONE).needsServer).toEqual([late]);
	});
});

describe('visibleClients', () => {
	it('lists every client when the box is empty, by accent-free name', () => {
		const all = [client('Mariana Solís'), client('Ána López'), client('Ana Rojas')];
		expect(visibleClients(all, '').map((c) => c.name)).toEqual([
			'Ána López',
			'Ana Rojas',
			'Mariana Solís'
		]);
	});

	it('lists more than the 20 results a search gives', () => {
		const all = Array.from({ length: 25 }, (_, i) =>
			client(`Cliente ${String(i).padStart(2, '0')}`)
		);
		expect(visibleClients(all, '   ')).toHaveLength(25);
	});

	it('finds "Ána López" before "Mariana Solís" for "ana"', () => {
		const all = [client('Mariana Solís'), client('Ána López')];
		expect(visibleClients(all, 'ana').map((c) => c.name)).toEqual(['Ána López', 'Mariana Solís']);
	});
});

describe('sharingNames', () => {
	it('names the clients that share the phone', () => {
		const error: ApiError = {
			kind: 'problem',
			status: 409,
			type: 'duplicate-phone',
			fields: {},
			problem: {
				type: 'urn:open-customer:problem:duplicate-phone',
				title: 'x',
				status: 409,
				clients: [client('Ana Rojas'), client('Lucía Rojas')]
			}
		};
		expect(sharingNames(error)).toEqual(['Ana Rojas', 'Lucía Rojas']);
	});

	it('is null for any other error', () => {
		expect(sharingNames({ kind: 'offline' })).toBeNull();
	});
});

describe('clientFormErrors', () => {
	it('asks for a name', () => {
		expect(clientFormErrors(form({ name: '  ' }))).toEqual({ name: 'client_name_needed' });
	});

	it('accepts a name', () => {
		expect(clientFormErrors(form())).toEqual({});
	});
});

describe('clientCreateBody', () => {
	it('sends only what she filled, trimmed', () => {
		const body = clientCreateBody(
			form({ name: ' Ana Rojas ', phone: ' 8888-7777 ', notes: ' Prefiere tardes ' }),
			DEFS
		);
		expect(body).toEqual({ name: 'Ana Rojas', phone: '8888-7777', notes: 'Prefiere tardes' });
	});

	it('leaves out an empty phone, empty notes and no attributes', () => {
		expect(clientCreateBody(form(), DEFS)).toEqual({ name: 'Ana Rojas' });
	});

	it('sends a number attribute as a number and keeps text as typed', () => {
		const body = clientCreateBody(form({ attrs: { alergias: 'látex', hijos: '2' } }), DEFS);
		expect(body.attrs).toEqual({ alergias: 'látex', hijos: 2 });
	});
});

describe('clientFormFrom', () => {
	it('fills the form from the client', () => {
		const saved = client('Ana Rojas', {
			phone_e164: '+50688887777',
			notes: 'Prefiere tardes',
			attrs: { alergias: 'látex', hijos: 2, gone: 'x' }
		});
		expect(clientFormFrom(saved, DEFS)).toEqual({
			name: 'Ana Rojas',
			phone: '+50688887777',
			notes: 'Prefiere tardes',
			attrs: { alergias: 'látex', hijos: '2' }
		});
	});
});

describe('clientPatchBody', () => {
	const before = client('Ana Rojas', {
		phone_e164: '+50688887777',
		notes: 'Nota',
		attrs: { alergias: 'látex' }
	});

	it('sends nothing when nothing changed', () => {
		expect(clientPatchBody(before, clientFormFrom(before, DEFS), DEFS)).toEqual({});
	});

	it('sends a changed name', () => {
		expect(
			clientPatchBody(before, { ...clientFormFrom(before, DEFS), name: 'Ana R.' }, DEFS)
		).toEqual({
			name: 'Ana R.'
		});
	});

	it('clears a phone and notes with null', () => {
		const cleared = { ...clientFormFrom(before, DEFS), phone: ' ', notes: '' };
		expect(clientPatchBody(before, cleared, DEFS)).toEqual({ phone: null, notes: null });
	});

	it('removes a cleared attribute with null', () => {
		const cleared = { ...clientFormFrom(before, DEFS), attrs: {} };
		expect(clientPatchBody(before, cleared, DEFS)).toEqual({ attrs: { alergias: null } });
	});
});

describe('restoreClientForm', () => {
	it('drops the attributes that no longer exist', () => {
		const saved = form({ attrs: { alergias: 'látex', gone: 'x' } });
		expect(restoreClientForm(saved, new Set(['alergias'])).attrs).toEqual({ alergias: 'látex' });
	});
});
