// General rules and the handover question in the rule logic (reminders-and-replies design, item 31).
import { describe, expect, it } from 'vitest';
import type { Rule, Service, Template } from './api/schemas';
import {
	ALL_SERVICES,
	handoverAsks,
	ruleRequest,
	rulePatch,
	ruleSentence,
	sortRules,
	type HandoverChange
} from './rules';

const NAILS = '11111111-1111-4111-8111-111111111111';
const BROWS = '22222222-2222-4222-8222-222222222222';
const COME_BACK = '44444444-4444-4444-8444-444444444444';
const OTHER = '55555555-5555-4555-8555-555555555555';
const SPECIFIC = '66666666-6666-4666-8666-666666666666';
const GENERAL = '77777777-7777-4777-8777-777777777777';

const rule = (id: string, over: Partial<Rule> = {}): Rule => ({
	id,
	kind: 'come_back',
	service_id: NAILS,
	return_service_id: null,
	offset_days: 21,
	template_id: COME_BACK,
	applies_from: null,
	archived: false,
	handover_from: null,
	...over
});
const general = (over: Partial<Rule> = {}) =>
	rule(GENERAL, { service_id: null, offset_days: 30, template_id: OTHER, ...over });
const service = (id: string, archived = false): Service => ({
	id,
	name: id === NAILS ? 'Uñas' : 'Cejas',
	description: null,
	duration_min: 60,
	price_cents: null,
	color: ['#', 'c76b85'].join(''),
	position: 0,
	archived
});
const template = (id: string, archived = false): Template => ({
	id,
	name: id === COME_BACK ? 'Volver' : 'Te extrañamos',
	body: 'Hola',
	archived
});

describe('ruleRequest for a general rule', () => {
	it('sends no service and no return service for Todos los servicios', () => {
		const form = {
			kind: 'come_back' as const,
			templateId: OTHER,
			days: '30',
			serviceId: ALL_SERVICES,
			returnId: ''
		};
		expect(ruleRequest(form)).toEqual({
			ok: true,
			body: { kind: 'come_back', template_id: OTHER, offset_days: 30 }
		});
	});
});

describe('rulePatch for a general rule', () => {
	it('never sends a return service', () => {
		expect(
			rulePatch(general(), { kind: 'come_back', template_id: OTHER, offset_days: 45 })
		).toEqual({ offset_days: 45 });
	});
});

describe('ruleSentence for a general rule', () => {
	it('says de la última visita', () => {
		expect(ruleSentence(general(), [template(OTHER)], [])).toBe(
			'Enviar Te extrañamos 30 días después de la última visita'
		);
	});
});

describe('sortRules with general rules', () => {
	it('puts general rules after the rules for one service, each by days', () => {
		const g45 = general({ id: '88888888-8888-4888-8888-888888888888', offset_days: 45 });
		const sorted = sortRules([g45, general(), rule(SPECIFIC, { offset_days: 60 })]);
		expect(sorted.map((r) => r.offset_days)).toEqual([60, 30, 45]);
	});
});

describe('handoverAsks', () => {
	const templates = [template(COME_BACK), template(OTHER)];
	const services = [service(NAILS), service(BROWS)];
	const rules = [rule(SPECIFIC), general()];
	const asks = (
		change: HandoverChange,
		over: Partial<{ r: Rule[]; t: Template[]; s: Service[] }> = {}
	) => handoverAsks(change, over.r ?? rules, over.t ?? templates, over.s ?? services);

	it('asks on the archive of a rule for one service that can send', () => {
		expect(asks({ kind: 'rule_archive', id: SPECIFIC })).toBe(true);
	});
	it('asks on the archive of its template', () => {
		expect(asks({ kind: 'template_archive', id: COME_BACK })).toBe(true);
	});
	it('asks on the archive of its service', () => {
		expect(asks({ kind: 'service_archive', id: NAILS })).toBe(true);
	});
	it('asks on the restore of the template that stopped it', () => {
		expect(
			asks(
				{ kind: 'template_restore', id: COME_BACK },
				{ t: [template(COME_BACK, true), template(OTHER)] }
			)
		).toBe(true);
	});
	it('asks on the restore of the service that stopped it', () => {
		expect(
			asks({ kind: 'service_restore', id: NAILS }, { s: [service(NAILS, true), service(BROWS)] })
		).toBe(true);
	});
	it('does not ask without a general rule', () => {
		expect(asks({ kind: 'rule_archive', id: SPECIFIC }, { r: [rule(SPECIFIC)] })).toBe(false);
	});
	it('does not ask when the general rule stops too', () => {
		const shared = [rule(SPECIFIC), general({ template_id: COME_BACK })];
		expect(asks({ kind: 'template_archive', id: COME_BACK }, { r: shared })).toBe(false);
	});
	it('does not ask when no rule for one service changes', () => {
		expect(asks({ kind: 'service_archive', id: BROWS })).toBe(false);
	});
	it('does not ask on the restore of a template while the service is still archived', () => {
		expect(
			asks(
				{ kind: 'template_restore', id: COME_BACK },
				{
					t: [template(COME_BACK, true), template(OTHER)],
					s: [service(NAILS, true), service(BROWS)]
				}
			)
		).toBe(false);
	});
});
