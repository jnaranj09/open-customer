import { describe, expect, it } from 'vitest';
import type { Appointment, AttrOption, Attribute, Service } from './api/schemas';
import {
	createBody,
	dayParam,
	attrDisplay,
	endFor,
	endShown,
	formErrors,
	formFrom,
	optionPath,
	patchBody,
	removeService,
	restoreForm,
	returnTo,
	serviceIds,
	statusActions,
	tapService,
	totalOf,
	withSavedLines,
	type Form,
	type Line
} from './booking';

const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const ACRILICO = '11111111-1111-4111-8111-111111111111';
const PEDICURE = '22222222-2222-4222-8222-222222222222';
const RELLENO = '33333333-3333-4333-8333-333333333333';
const ZONE = 'America/Costa_Rica';

const service = (id: string, name: string, duration_min: number, price_cents: number | null) =>
	({
		id,
		name,
		duration_min,
		price_cents,
		color: 'rose',
		position: 0,
		archived: false
	}) satisfies Service;

const SERVICES = [
	service(ACRILICO, 'Acrílico', 90, 2000000),
	service(PEDICURE, 'Pedicure', 60, 1000000),
	service(RELLENO, 'Relleno', 45, null)
];

const FORMA = '44444444-4444-4444-8444-444444444444';
const option = (id: string, label: string, parent_id: string | null, archived = false) =>
	({ id, attribute_id: FORMA, parent_id, label, position: 0, archived }) satisfies AttrOption;

const ALMENDRA = '55555555-5555-4555-8555-555555555555';
const CORTA = '66666666-6666-4666-8666-666666666666';
const CUADRADA = '77777777-7777-4777-8777-777777777777';
const OPTIONS = [
	option(ALMENDRA, 'Almendra', null),
	option(CORTA, 'corta', ALMENDRA),
	option(CUADRADA, 'Cuadrada', null)
];

const attribute = (
	key: string,
	type: Attribute['type'],
	over: Partial<Attribute> = {}
): Attribute => ({
	id: ID,
	key,
	label: key,
	scope: 'appointment',
	type,
	required: false,
	position: 0,
	archived: false,
	options: [],
	...over
});

const DEFS = [
	attribute('forma', 'select', { options: OPTIONS }),
	attribute('nota', 'text'),
	attribute('tamano', 'number'),
	attribute('colores', 'multiselect', { options: OPTIONS })
];

const form = (over: Partial<Form> = {}): Form => ({
	clientId: ID,
	lines: [{ service_id: ACRILICO, count: 1 }],
	day: '2026-11-07',
	time: '15:00',
	end: null,
	notes: '',
	attrs: {},
	...over
});

describe('tapService', () => {
	it('adds a second copy on a second tap', () => {
		const lines = tapService(tapService([], ACRILICO), ACRILICO);
		expect(lines).toEqual([{ service_id: ACRILICO, count: 2 }]);
	});

	it('keeps the order of the first taps', () => {
		const lines = tapService(tapService(tapService([], ACRILICO), PEDICURE), ACRILICO);
		expect(lines.map((l) => l.service_id)).toEqual([ACRILICO, PEDICURE]);
	});
});

describe('removeService', () => {
	it('removes one copy', () => {
		const lines: Line[] = [{ service_id: ACRILICO, count: 2 }];
		expect(removeService(lines, ACRILICO)).toEqual([{ service_id: ACRILICO, count: 1 }]);
	});

	it('drops the line when the last copy goes', () => {
		const lines: Line[] = [{ service_id: ACRILICO, count: 1 }];
		expect(removeService(lines, ACRILICO)).toEqual([]);
	});
});

describe('serviceIds', () => {
	it('repeats a service picked twice', () => {
		const lines: Line[] = [
			{ service_id: ACRILICO, count: 2 },
			{ service_id: PEDICURE, count: 1 }
		];
		expect(serviceIds(lines)).toEqual([ACRILICO, ACRILICO, PEDICURE]);
	});
});

describe('endFor', () => {
	it('sums the durations', () => {
		const lines: Line[] = [
			{ service_id: ACRILICO, count: 1 },
			{ service_id: PEDICURE, count: 1 }
		];
		expect(endFor('15:00', lines, SERVICES)).toBe('17:30');
	});

	it('counts a repeated service once per copy', () => {
		expect(endFor('15:00', [{ service_id: PEDICURE, count: 2 }], SERVICES)).toBe('17:00');
	});

	it('gives nothing for an empty start', () => {
		expect(endFor('', [{ service_id: PEDICURE, count: 1 }], SERVICES)).toBe('');
	});

	it('keeps the start with no services', () => {
		expect(endFor('15:00', [], SERVICES)).toBe('15:00');
	});

	it('wraps past midnight', () => {
		expect(endFor('23:30', [{ service_id: PEDICURE, count: 1 }], SERVICES)).toBe('00:30');
	});
});

describe('totalOf', () => {
	it('sums the prices, a service without a price counting zero', () => {
		const lines: Line[] = [
			{ service_id: ACRILICO, count: 1 },
			{ service_id: PEDICURE, count: 2 },
			{ service_id: RELLENO, count: 1 }
		];
		expect(totalOf(lines, SERVICES)).toBe(4000000);
	});
});

describe('optionPath', () => {
	it('returns the chain to a nested option', () => {
		expect(optionPath(OPTIONS, CORTA).map((o) => o.label)).toEqual(['Almendra', 'corta']);
	});

	it('returns just the option at the top level', () => {
		expect(optionPath(OPTIONS, CUADRADA).map((o) => o.label)).toEqual(['Cuadrada']);
	});

	it('returns nothing for no pick', () => {
		expect(optionPath(OPTIONS, null)).toEqual([]);
	});

	it('returns nothing for an unknown id', () => {
		expect(optionPath(OPTIONS, ID)).toEqual([]);
	});
});

describe('returnTo', () => {
	it('defaults to Hoy', () => {
		expect(returnTo(null)).toBe('/');
	});

	it('keeps an app path', () => {
		expect(returnTo('/clientas/abc')).toBe('/clientas/abc');
	});

	it('refuses a path that leaves the app', () => {
		expect(returnTo('//evil.test/x')).toBe('/');
	});

	it('refuses a path with a tab that the URL parser strips', () => {
		expect(returnTo('/\t/evil.test/x')).toBe('/');
	});

	it('refuses a path with a backslash', () => {
		expect(returnTo('/\\evil.test')).toBe('/');
	});

	it('keeps the search of an app path', () => {
		expect(returnTo('/clientas?q=ana')).toBe('/clientas?q=ana');
	});

	it('refuses an absolute URL', () => {
		expect(returnTo('https://evil.test')).toBe('/');
	});
});

describe('dayParam', () => {
	it('keeps a real day', () => {
		expect(dayParam('2026-11-07', '2026-11-04')).toBe('2026-11-07');
	});

	it('falls back to today for nothing', () => {
		expect(dayParam(null, '2026-11-04')).toBe('2026-11-04');
	});

	it('falls back to today for a day that does not exist', () => {
		expect(dayParam('2026-13-45', '2026-11-04')).toBe('2026-11-04');
	});
});

describe('formErrors', () => {
	it('accepts a complete form', () => {
		expect(formErrors(form(), true)).toEqual({});
	});

	it('asks for a client when creating', () => {
		expect(formErrors(form({ clientId: null }), true)).toEqual({
			client: 'appointment_client_needed'
		});
	});

	it('does not ask for a client when editing', () => {
		expect(formErrors(form({ clientId: null }), false)).toEqual({});
	});

	it('asks for the time', () => {
		expect(formErrors(form({ time: '' }), true)).toEqual({ time: 'appointment_time_needed' });
	});

	it('does not ask for an end time when editing an appointment that never had services', () => {
		expect(formErrors(form({ lines: [] }), false, false)).toEqual({});
	});

	it('asks for an end time when an edit removes the last service', () => {
		expect(formErrors(form({ lines: [] }), false, true)).toEqual({ end: 'appointment_end_needed' });
	});

	it('refuses a day that is not a date', () => {
		expect(formErrors(form({ day: 'x' }), true)).toEqual({ day: 'appointment_day_needed' });
	});

	it('asks for an end time when there are no services', () => {
		expect(formErrors(form({ lines: [] }), true)).toEqual({ end: 'appointment_end_needed' });
	});

	it('accepts an end time with no services', () => {
		expect(formErrors(form({ lines: [], end: '16:00' }), true)).toEqual({});
	});

	it('refuses an end time that is not after the start', () => {
		expect(formErrors(form({ end: '15:00' }), true)).toEqual({ end: 'appointment_end_after' });
	});
});

describe('createBody', () => {
	it('copies the day, time and services into the request', () => {
		const lines: Line[] = [
			{ service_id: ACRILICO, count: 1 },
			{ service_id: PEDICURE, count: 1 }
		];
		expect(createBody(form({ lines }), ZONE, DEFS)).toEqual({
			client_id: ID,
			starts_at: '2026-11-07T21:00:00Z',
			service_ids: [ACRILICO, PEDICURE]
		});
	});

	it('sends the end time only when she set it', () => {
		expect(createBody(form({ lines: [], end: '16:00' }), ZONE, DEFS)).toEqual({
			client_id: ID,
			starts_at: '2026-11-07T21:00:00Z',
			ends_at: '2026-11-07T22:00:00Z'
		});
	});

	it('sends the notes trimmed and leaves out empty ones', () => {
		expect(createBody(form({ notes: '  traer diseño ' }), ZONE, DEFS).notes).toBe('traer diseño');
	});

	it('leaves out blank notes', () => {
		expect(createBody(form({ notes: '   ' }), ZONE, DEFS)).not.toHaveProperty('notes');
	});

	it('sends the attribute values by key and drops empty ones', () => {
		const attrs = { forma: CORTA, nota: '  ', tamano: '12,5', colores: [ALMENDRA, CUADRADA] };
		expect(createBody(form({ attrs }), ZONE, DEFS).attrs).toEqual({
			forma: CORTA,
			tamano: 12.5,
			colores: [ALMENDRA, CUADRADA]
		});
	});

	it('leaves out attrs when none has a value', () => {
		expect(createBody(form({ attrs: { forma: '' } }), ZONE, DEFS)).not.toHaveProperty('attrs');
	});
});

const appointment = (over: Partial<Appointment> = {}): Appointment => ({
	id: ID,
	client_id: ID,
	client_name: 'Ána López',
	starts_at: '2026-11-07T21:00:00Z',
	ends_at: '2026-11-07T22:30:00Z',
	status: 'scheduled',
	done: false,
	total_cents: 2000000,
	paid_cents: null,
	attrs: { forma: CORTA, nota: 'sin brillo' },
	notes: 'traer diseño',
	services: [{ service_id: ACRILICO, name: 'Acrílico', price_cents: 2000000, duration_min: 90 }],
	created_at: '2026-11-01T10:00:00Z',
	updated_at: '2026-11-01T10:00:00Z',
	...over
});

describe('formFrom', () => {
	it('reads the appointment back into the form in the business zone', () => {
		expect(formFrom(appointment(), ZONE, DEFS)).toEqual({
			clientId: ID,
			lines: [{ service_id: ACRILICO, count: 1 }],
			day: '2026-11-07',
			time: '15:00',
			end: null,
			notes: 'traer diseño',
			attrs: { forma: CORTA, nota: 'sin brillo' }
		});
	});

	it('groups a service booked twice into one line', () => {
		const line = { service_id: PEDICURE, name: 'Pedicure', price_cents: 1000000, duration_min: 60 };
		const got = formFrom(appointment({ services: [line, line] }), ZONE, DEFS);
		expect(got.lines).toEqual([{ service_id: PEDICURE, count: 2 }]);
	});

	it('writes a number back as text', () => {
		const got = formFrom(appointment({ attrs: { tamano: 12.5 } }), ZONE, DEFS);
		expect(got.attrs).toEqual({ tamano: '12.5' });
	});
});

describe('patchBody', () => {
	const before = appointment();
	const same = (): Form => formFrom(before, ZONE, DEFS);

	it('is empty when nothing changed', () => {
		expect(patchBody(before, same(), ZONE, DEFS)).toEqual({});
	});

	it('sends only the start when the day moved, so the server moves the end with it', () => {
		const got = patchBody(before, { ...same(), day: '2026-11-08' }, ZONE, DEFS);
		expect(got).toEqual({ starts_at: '2026-11-08T21:00:00Z' });
	});

	it('sends service_ids, not ends_at, when the services changed', () => {
		const lines: Line[] = [
			{ service_id: ACRILICO, count: 1 },
			{ service_id: PEDICURE, count: 1 }
		];
		expect(patchBody(before, { ...same(), lines }, ZONE, DEFS)).toEqual({
			service_ids: [ACRILICO, PEDICURE]
		});
	});

	it('sends the end time she set', () => {
		expect(patchBody(before, { ...same(), end: '17:00' }, ZONE, DEFS)).toEqual({
			ends_at: '2026-11-07T23:00:00Z'
		});
	});

	it('keeps the end she set when the start moved, even if it equals the saved end', () => {
		const got = patchBody(before, { ...same(), time: '14:00', end: '16:30' }, ZONE, DEFS);
		expect(got).toEqual({ starts_at: '2026-11-07T20:00:00Z', ends_at: '2026-11-07T22:30:00Z' });
	});

	it('leaves out an end time equal to the saved one', () => {
		expect(patchBody(before, { ...same(), end: '16:30' }, ZONE, DEFS)).toEqual({});
	});

	it('clears the notes with null', () => {
		expect(patchBody(before, { ...same(), notes: ' ' }, ZONE, DEFS)).toEqual({ notes: null });
	});

	it('sends a changed attribute and null for a cleared one', () => {
		const attrs = { forma: CUADRADA, nota: '' };
		expect(patchBody(before, { ...same(), attrs }, ZONE, DEFS)).toEqual({
			attrs: { forma: CUADRADA, nota: null }
		});
	});
});

describe('withSavedLines', () => {
	it('takes the saved duration and price over the current one for a service on the appointment', () => {
		const saved = [
			{ service_id: PEDICURE, name: 'Pedicure', price_cents: 800000, duration_min: 50 }
		];
		const got = withSavedLines(SERVICES, saved).find((s) => s.id === PEDICURE);
		expect([got?.duration_min, got?.price_cents]).toEqual([50, 800000]);
	});

	it('adds a service that was archived since, marked archived', () => {
		const gone = '88888888-8888-4888-8888-888888888888';
		const saved = [{ service_id: gone, name: 'Keratina', price_cents: null, duration_min: 30 }];
		const got = withSavedLines(SERVICES, saved).find((s) => s.id === gone);
		expect([got?.name, got?.archived]).toEqual(['Keratina', true]);
	});
});

describe('endShown', () => {
	const booked = appointment();
	const untouched = (): Form => formFrom(booked, ZONE, DEFS);
	const quote = withSavedLines(SERVICES, booked.services);

	it('shows the end she set', () => {
		expect(endShown({ ...untouched(), end: '18:00' }, booked, ZONE, quote)).toBe('18:00');
	});

	it('shows the saved end of an appointment she has not moved', () => {
		expect(endShown(untouched(), booked, ZONE, quote)).toBe('16:30');
	});

	it('moves the saved end with the start', () => {
		expect(endShown({ ...untouched(), time: '16:00' }, booked, ZONE, quote)).toBe('17:30');
	});

	it('adds up the durations once the services changed', () => {
		const lines: Line[] = [{ service_id: PEDICURE, count: 1 }];
		expect(endShown({ ...untouched(), lines }, booked, ZONE, quote)).toBe('16:00');
	});

	it('does not throw when the day was cleared on an edit', () => {
		expect(endShown({ ...untouched(), day: '' }, booked, ZONE, quote)).toBe('16:30');
	});

	it('shows nothing for a cleared time', () => {
		expect(endShown({ ...untouched(), time: '' }, booked, ZONE, quote)).toBe('');
	});

	it('adds up the durations for a new appointment', () => {
		expect(endShown(form(), null, ZONE, quote)).toBe('16:30');
	});
});

describe('restoreForm', () => {
	const known = {
		clients: new Set([ID]),
		services: new Set([ACRILICO]),
		attrKeys: new Set(['forma'])
	};
	const saved = (): Form =>
		form({
			clientId: ID,
			lines: [
				{ service_id: ACRILICO, count: 1 },
				{ service_id: PEDICURE, count: 1 }
			],
			attrs: { forma: CORTA, viejo: 'x' }
		});

	it('keeps what is still there', () => {
		const got = restoreForm(saved(), known);
		expect([got.clientId, got.day, got.time]).toEqual([ID, '2026-11-07', '15:00']);
	});

	it('drops a client that is gone', () => {
		const got = restoreForm(saved(), { ...known, clients: new Set() });
		expect(got.clientId).toBeNull();
	});

	it('drops a service that is gone', () => {
		expect(restoreForm(saved(), known).lines).toEqual([{ service_id: ACRILICO, count: 1 }]);
	});

	it('drops the value of an attribute that is gone', () => {
		expect(restoreForm(saved(), known).attrs).toEqual({ forma: CORTA });
	});
});

describe('statusActions', () => {
	it('offers confirm, cancel and no-show on a scheduled appointment', () => {
		expect(statusActions('scheduled')).toEqual(['confirmed', 'cancelled', 'no_show']);
	});

	it('offers cancel and no-show on a confirmed one', () => {
		expect(statusActions('confirmed')).toEqual(['cancelled', 'no_show']);
	});

	it('offers only reactivate on a cancelled one', () => {
		expect(statusActions('cancelled')).toEqual(['scheduled']);
	});

	it('offers only reactivate on a no-show', () => {
		expect(statusActions('no_show')).toEqual(['scheduled']);
	});
});

describe('attrDisplay', () => {
	it('writes a nested pick as its labels from the top, joined by a space', () => {
		expect(attrDisplay(DEFS[0] as Attribute, CORTA)).toBe('Almendra corta');
	});

	it('writes a multiselect as its picks joined by a comma', () => {
		expect(attrDisplay(DEFS[3] as Attribute, [ALMENDRA, CUADRADA])).toBe('Almendra, Cuadrada');
	});

	it('writes a number as given', () => {
		expect(attrDisplay(DEFS[2] as Attribute, 12.5)).toBe('12.5');
	});

	it('writes a date in words', () => {
		const date = attribute('fecha', 'date');
		expect(attrDisplay(date, '2026-11-07')).toBe('sábado 7 de noviembre');
	});

	it('has nothing to show for an empty value', () => {
		expect(attrDisplay(DEFS[1] as Attribute, '')).toBeNull();
	});

	it('has nothing to show for no value', () => {
		expect(attrDisplay(DEFS[1] as Attribute, undefined)).toBeNull();
	});
});
