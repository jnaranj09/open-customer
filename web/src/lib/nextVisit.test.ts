import { describe, expect, it } from 'vitest';
import type { Appointment, Rule, Service, Template } from './api/schemas';
import type { Line } from './booking';
import { canBookNext, choiceTitle, linesText, nextVisit } from './nextVisit';

const ACRYLIC = '11111111-1111-4111-8111-111111111111';
const REFILL = '22222222-2222-4222-8222-222222222222';
const PEDICURE = '33333333-3333-4333-8333-333333333333';
const REMINDER = '44444444-4444-4444-8444-444444444444';
const MISS_YOU = '55555555-5555-4555-8555-555555555555';
const ZONE = 'America/Costa_Rica';
const TODAY = '2026-11-12';
/** 16:00 on 12 Nov in the business zone. */
const ENDS = '2026-11-12T22:00:00Z';

let nextId = 0;
/** A come-back rule "21 días después de Acrílico → Relleno"; each call gets a later id. */
function rule(over: Partial<Rule> = {}): Rule {
	nextId += 1;
	return {
		id: `00000000-0000-4000-8000-${String(nextId).padStart(12, '0')}`,
		kind: 'come_back',
		service_id: ACRYLIC,
		return_service_id: REFILL,
		offset_days: 21,
		template_id: REMINDER,
		applies_from: null,
		archived: false,
		handover_from: null,
		...over
	};
}
/** A rule that returns to its own service. */
const own = (service_id: string | null, offset_days: number): Rule =>
	rule({ service_id, return_service_id: null, offset_days });

const service = (id: string, name: string, archived = false): Service => ({
	id,
	name,
	description: null,
	duration_min: 60,
	price_cents: null,
	color: ['#', 'c76b85'].join(''),
	position: 0,
	archived
});
const NAMES: Record<string, string> = {
	[ACRYLIC]: 'Acrílico',
	[REFILL]: 'Relleno',
	[PEDICURE]: 'Pedicure'
};
const services = Object.entries(NAMES).map(([id, name]) => service(id, name));
const templates: Template[] = [{ id: REMINDER, name: 'Relleno', body: 'Hola', archived: false }];

/** A visit with one copy of each service id, ending at `ends_at`. */
const appt = (ids: string[], ends_at = ENDS): Pick<Appointment, 'ends_at' | 'services'> => ({
	ends_at,
	services: ids.map((service_id) => ({
		service_id,
		name: NAMES[service_id] ?? '',
		price_cents: null,
		duration_min: 60
	}))
});
const line = (service_id: string, count = 1): Line => ({ service_id, count });

type Given = { rules: Rule[]; visit: string[]; today?: string; ends?: string };
const run = (given: Given, catalog: Service[] = services, tpls: Template[] = templates) =>
	nextVisit(appt(given.visit, given.ends), given.rules, tpls, catalog, ZONE, given.today ?? TODAY);

describe('nextVisit', () => {
	it('a before rule gives no choice', () => {
		const before = rule({ kind: 'before_appointment', service_id: null, return_service_id: null });

		expect(run({ rules: [before], visit: [ACRYLIC] })).toEqual({
			choices: [],
			otherDate: [line(ACRYLIC)]
		});
	});

	it('a rule for a service on the visit', () => {
		expect(run({ rules: [rule()], visit: [ACRYLIC] })).toEqual({
			choices: [{ days: 21, day: '2026-12-03', lines: [line(REFILL)] }],
			otherDate: [line(REFILL)]
		});
	});

	it('a rule for another service does not count', () => {
		expect(run({ rules: [own(PEDICURE, 30)], visit: [ACRYLIC] }).choices).toEqual([]);
	});

	it('a general rule alone', () => {
		expect(run({ rules: [own(null, 30)], visit: [PEDICURE] }).choices).toEqual([
			{ days: 30, day: '2026-12-12', lines: [line(PEDICURE)] }
		]);
	});

	it('a general rule is left out when a specific one counts', () => {
		const got = run({ rules: [own(null, 30), rule()], visit: [ACRYLIC] });

		expect(got.choices.map((c) => c.days)).toEqual([21]);
	});

	it.each([
		['the rule is archived', { archivedRule: true }],
		['its template is archived', { archivedTemplate: true }],
		['its service is archived', { archivedService: ACRYLIC }],
		['its return service is archived', { archivedService: REFILL }]
	] as const)('a rule that cannot send does not count: %s', (_name, off) => {
		const specific = rule({ archived: 'archivedRule' in off });
		const catalog = services.map((s) =>
			'archivedService' in off && s.id === off.archivedService ? { ...s, archived: true } : s
		);
		const tpls = templates.map((tpl) => ({ ...tpl, archived: 'archivedTemplate' in off }));
		// The general rule has its own template, so it still sends when the other one is archived.
		const general = rule({
			service_id: null,
			return_service_id: null,
			offset_days: 30,
			template_id: MISS_YOU
		});
		const other = { id: MISS_YOU, name: 'Te extrañamos', body: 'Hola', archived: false };

		const got = run({ rules: [specific, general], visit: [ACRYLIC] }, catalog, [...tpls, other]);

		expect(got.choices.map((c) => c.days)).toEqual([30]);
	});

	it('CUJ N1: two rules, one appointment', () => {
		const got = run({ rules: [rule(), own(PEDICURE, 30)], visit: [ACRYLIC, PEDICURE] });

		expect(got.choices).toEqual([
			{ days: 21, day: '2026-12-03', lines: [line(REFILL), line(PEDICURE)] },
			{ days: 30, day: '2026-12-12', lines: [line(REFILL), line(PEDICURE)] }
		]);
	});

	it('two rules with the same days give one choice', () => {
		const got = run({ rules: [rule(), own(PEDICURE, 21)], visit: [ACRYLIC, PEDICURE] });

		expect(got.choices.map((c) => c.days)).toEqual([21]);
	});

	it('CUJ N3b: the services follow the picked days', () => {
		const got = run({ rules: [rule(), own(ACRYLIC, 60)], visit: [ACRYLIC] });

		expect(got.choices).toEqual([
			{ days: 21, day: '2026-12-03', lines: [line(REFILL)] },
			{ days: 60, day: '2027-01-11', lines: [line(ACRYLIC)] }
		]);
		expect(got.otherDate).toEqual([line(REFILL)]);
	});

	it('the fewest days decide for a service with no rule of the picked days', () => {
		const got = run({
			rules: [own(ACRYLIC, 60), rule(), own(PEDICURE, 30)],
			visit: [ACRYLIC, PEDICURE]
		});

		expect(got.choices.find((c) => c.days === 30)?.lines).toEqual([line(REFILL), line(PEDICURE)]);
	});

	it('two rules of the same days: the lower id decides', () => {
		const lower = rule({ return_service_id: PEDICURE });
		const higher = rule();

		const got = run({ rules: [higher, lower], visit: [ACRYLIC] });

		expect(got.choices).toEqual([{ days: 21, day: '2026-12-03', lines: [line(PEDICURE)] }]);
	});

	it('the count is kept', () => {
		const got = run({ rules: [rule()], visit: [ACRYLIC, ACRYLIC] });

		expect(got.otherDate).toEqual([line(REFILL, 2)]);
	});

	it('two lines that become one service keep the larger count', () => {
		const got = run({ rules: [rule()], visit: [ACRYLIC, ACRYLIC, REFILL] });

		expect(got.otherDate).toEqual([line(REFILL, 2)]);
	});

	it('an archived service is left out', () => {
		const catalog = services.map((s) => (s.id === PEDICURE ? { ...s, archived: true } : s));

		expect(run({ rules: [], visit: [ACRYLIC, PEDICURE] }, catalog).otherDate).toEqual([
			line(ACRYLIC)
		]);
	});

	it('a choice before today is left out, today stays', () => {
		const given = { rules: [rule()], visit: [ACRYLIC] };

		expect(run({ ...given, today: '2026-12-04' }).choices).toEqual([]);
		expect(run({ ...given, today: '2026-12-03' }).choices.map((c) => c.day)).toEqual([
			'2026-12-03'
		]);
	});

	it('counts from the local day the visit ends', () => {
		const given = { rules: [rule()], visit: [ACRYLIC] };
		// 21:30 on the 12th, then 00:30 on the 13th (a visit that runs past midnight).
		const evening = run({ ...given, ends: '2026-11-13T03:30:00Z' });
		const pastMidnight = run({ ...given, ends: '2026-11-13T06:30:00Z' });

		expect(evening.choices.map((c) => c.day)).toEqual(['2026-12-03']);
		expect(pastMidnight.choices.map((c) => c.day)).toEqual(['2026-12-04']);
	});

	it('applies_from and handover_from are not read', () => {
		const later = '2027-01-01T00:00:00Z';
		const got = run({
			rules: [rule({ applies_from: later, handover_from: later })],
			visit: [ACRYLIC]
		});

		expect(got.choices.map((c) => c.days)).toEqual([21]);
	});

	it('an appointment with no services', () => {
		expect(run({ rules: [own(null, 30)], visit: [] })).toEqual({
			choices: [{ days: 30, day: '2026-12-12', lines: [] }],
			otherDate: []
		});
	});
});

describe('choiceTitle', () => {
	it('names the days and the date', () => {
		expect(choiceTitle({ days: 21, day: '2026-12-03', lines: [] })).toBe('En 21 días · 3 dic');
	});

	it('says día for one day', () => {
		expect(choiceTitle({ days: 1, day: '2026-12-04', lines: [] })).toBe('En 1 día · 4 dic');
	});
});

describe('linesText', () => {
	it('joins the names, with the count above one', () => {
		expect(linesText([line(REFILL, 2), line(PEDICURE)], services)).toBe('Relleno ×2, Pedicure');
	});

	it('says Sin servicios for no line', () => {
		expect(linesText([], services)).toBe('Sin servicios');
	});

	it('leaves out a service it does not know', () => {
		expect(linesText([line('unknown'), line(PEDICURE)], services)).toBe('Pedicure');
	});
});

describe('canBookNext', () => {
	const clients = [{ id: 'client-ana' }];

	it.each([
		['scheduled', true],
		['confirmed', true],
		['cancelled', false],
		['no_show', false]
	] as const)('a %s appointment: %s', (status, want) => {
		expect(canBookNext({ status, client_id: 'client-ana' }, clients)).toBe(want);
	});

	it('is false for a client that is not in the list (archived)', () => {
		expect(canBookNext({ status: 'scheduled', client_id: 'client-eva' }, clients)).toBe(false);
	});
});
