import { describe, expect, it } from 'vitest';
import type { Rule, Service, Template } from './api/schemas';
import {
	overdueChoice,
	offsetText,
	ruleRequest,
	rulePatch,
	ruleSentence,
	ruleWarnings,
	sortRules,
	type RuleForm
} from './rules';

const RULE_ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const ACRYLIC = '11111111-1111-4111-8111-111111111111';
const REFILL = '22222222-2222-4222-8222-222222222222';
const PEDICURE = '33333333-3333-4333-8333-333333333333';
const REMINDER = '44444444-4444-4444-8444-444444444444';
const OTHER = '55555555-5555-4555-8555-555555555555';

function rule(over: Partial<Rule> = {}): Rule {
	return {
		id: RULE_ID,
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
const template = (id: string, name: string, archived = false): Template => ({
	id,
	name,
	body: 'Hola',
	archived
});

const services = [service(ACRYLIC, 'Acrílico'), service(REFILL, 'Relleno')];
const templates = [template(REMINDER, 'Relleno'), template(OTHER, 'Recordatorio de cita')];

describe('offsetText', () => {
	it('says el mismo día for 0', () => {
		expect(offsetText('before_appointment', 0)).toBe('el mismo día');
	});
	it('says 1 día antes', () => {
		expect(offsetText('before_appointment', 1)).toBe('1 día antes');
	});
	it('says 2 días antes', () => {
		expect(offsetText('before_appointment', 2)).toBe('2 días antes');
	});
	it('says 1 día después', () => {
		expect(offsetText('come_back', 1)).toBe('1 día después');
	});
	it('says 21 días después', () => {
		expect(offsetText('come_back', 21)).toBe('21 días después');
	});
});

describe('overdueChoice', () => {
	it('asks for a new come-back rule', () => {
		expect(
			overdueChoice(null, {
				kind: 'come_back',
				offset_days: 21,
				template_id: OTHER,
				service_id: ACRYLIC
			})
		).toBe('ask');
	});
	it('includes without asking for a new before rule', () => {
		expect(
			overdueChoice(null, { kind: 'before_appointment', offset_days: 1, template_id: OTHER })
		).toBe('include');
	});
	it('includes without asking when a before rule changes its offset', () => {
		expect(
			overdueChoice(rule({ kind: 'before_appointment', service_id: null }), { offset_days: 2 })
		).toBe('include');
	});
	it('needs nothing when a before rule changes nothing that adds items', () => {
		expect(
			overdueChoice(rule({ kind: 'before_appointment', service_id: null }), { archived: true })
		).toBe('none');
	});
	it('asks when the offset changes', () => {
		expect(overdueChoice(rule(), { offset_days: 14 })).toBe('ask');
	});
	it('needs nothing when the offset is sent unchanged', () => {
		expect(overdueChoice(rule(), { offset_days: 21 })).toBe('none');
	});
	it('asks when the effective return service changes', () => {
		expect(overdueChoice(rule(), { return_service_id: PEDICURE })).toBe('ask');
	});
	it('asks when a null return becomes another service', () => {
		expect(overdueChoice(rule({ return_service_id: null }), { return_service_id: REFILL })).toBe(
			'ask'
		);
	});
	it("needs nothing for setting return to null when it was the rule's own service", () => {
		expect(overdueChoice(rule({ return_service_id: ACRYLIC }), { return_service_id: null })).toBe(
			'none'
		);
	});
	it('asks for setting return to null when it was another service', () => {
		expect(overdueChoice(rule(), { return_service_id: null })).toBe('ask');
	});
	it('needs nothing for a template change when the old template is active', () => {
		expect(overdueChoice(rule(), { template_id: OTHER })).toBe('none');
	});
	it('asks for a template change when the old template is archived', () => {
		expect(overdueChoice(rule(), { template_id: OTHER }, new Set([REMINDER]))).toBe('ask');
	});
	it('asks when an archived rule is restored', () => {
		expect(overdueChoice(rule({ archived: true }), { archived: false })).toBe('ask');
	});
	it('needs nothing when a rule is archived', () => {
		expect(overdueChoice(rule(), { archived: true })).toBe('none');
	});
});

const comeBackForm: RuleForm = {
	kind: 'come_back',
	templateId: REMINDER,
	days: ' 21 ',
	serviceId: ACRYLIC,
	returnId: REFILL
};

describe('ruleRequest', () => {
	it('builds a come-back request with its service and return service', () => {
		expect(ruleRequest(comeBackForm)).toEqual({
			ok: true,
			body: {
				kind: 'come_back',
				template_id: REMINDER,
				offset_days: 21,
				service_id: ACRYLIC,
				return_service_id: REFILL
			}
		});
	});
	it('leaves the return service out when it is the same service', () => {
		expect(ruleRequest({ ...comeBackForm, returnId: '' })).toEqual({
			ok: true,
			body: { kind: 'come_back', template_id: REMINDER, offset_days: 21, service_id: ACRYLIC }
		});
	});
	it('builds a before request with no service', () => {
		expect(
			ruleRequest({ ...comeBackForm, kind: 'before_appointment', days: '0', serviceId: ACRYLIC })
		).toEqual({
			ok: true,
			body: { kind: 'before_appointment', template_id: REMINDER, offset_days: 0 }
		});
	});
	it('asks for the template, the days and the service', () => {
		expect(ruleRequest({ ...comeBackForm, templateId: '', days: 'x', serviceId: '' })).toEqual({
			ok: false,
			errors: {
				template: 'rule_template_required',
				days: 'rule_days_invalid_come_back',
				service: 'rule_service_required'
			}
		});
	});
	it('refuses 0 days for a come-back rule', () => {
		expect(ruleRequest({ ...comeBackForm, days: '0' })).toEqual({
			ok: false,
			errors: { days: 'rule_days_invalid_come_back' }
		});
	});
	it('refuses more than 365 days for a before rule', () => {
		expect(ruleRequest({ ...comeBackForm, kind: 'before_appointment', days: '366' })).toEqual({
			ok: false,
			errors: { days: 'rule_days_invalid_before' }
		});
	});
});

describe('rulePatch', () => {
	it('sends nothing when nothing changed', () => {
		const body = {
			kind: 'come_back',
			template_id: REMINDER,
			offset_days: 21,
			service_id: ACRYLIC,
			return_service_id: REFILL
		} as const;
		expect(rulePatch(rule(), body)).toEqual({});
	});
	it('sends the changed offset and template', () => {
		const body = {
			kind: 'come_back',
			template_id: OTHER,
			offset_days: 14,
			service_id: ACRYLIC,
			return_service_id: REFILL
		} as const;
		expect(rulePatch(rule(), body)).toEqual({ offset_days: 14, template_id: OTHER });
	});
	it('sends a null return service when it goes back to the own service', () => {
		const body = {
			kind: 'come_back',
			template_id: REMINDER,
			offset_days: 21,
			service_id: ACRYLIC
		} as const;
		expect(rulePatch(rule(), body)).toEqual({ return_service_id: null });
	});
	it('sends no return service when the effective one is the same', () => {
		const body = {
			kind: 'come_back',
			template_id: REMINDER,
			offset_days: 21,
			service_id: ACRYLIC
		} as const;
		expect(rulePatch(rule({ return_service_id: ACRYLIC }), body)).toEqual({});
	});
});

describe('ruleSentence', () => {
	it('writes a come-back rule', () => {
		expect(ruleSentence(rule(), templates, services)).toBe(
			'Enviar Relleno 21 días después de Acrílico, para agendar Relleno'
		);
	});
	it('names the rule service when the return service is empty', () => {
		expect(ruleSentence(rule({ return_service_id: null }), templates, services)).toBe(
			'Enviar Relleno 21 días después de Acrílico, para agendar Acrílico'
		);
	});
	it('writes a before rule', () => {
		expect(
			ruleSentence(
				rule({
					kind: 'before_appointment',
					service_id: null,
					return_service_id: null,
					offset_days: 1,
					template_id: OTHER
				}),
				templates,
				services
			)
		).toBe('Enviar Recordatorio de cita 1 día antes de cada cita');
	});
	it('writes the same day', () => {
		expect(
			ruleSentence(
				rule({
					kind: 'before_appointment',
					service_id: null,
					return_service_id: null,
					offset_days: 0,
					template_id: OTHER
				}),
				templates,
				services
			)
		).toBe('Enviar Recordatorio de cita el mismo día de cada cita');
	});
});

describe('ruleWarnings', () => {
	it('is empty when every part is active', () => {
		expect(ruleWarnings(rule(), templates, services)).toEqual([]);
	});
	it('warns about an archived template', () => {
		expect(ruleWarnings(rule(), [template(REMINDER, 'Relleno', true)], services)).toEqual([
			'rule_warn_template'
		]);
	});
	it('warns about an archived service', () => {
		expect(
			ruleWarnings(rule(), templates, [
				service(ACRYLIC, 'Acrílico', true),
				service(REFILL, 'Relleno')
			])
		).toEqual(['rule_warn_service']);
	});
	it('warns about an archived return service', () => {
		expect(
			ruleWarnings(rule(), templates, [
				service(ACRYLIC, 'Acrílico'),
				service(REFILL, 'Relleno', true)
			])
		).toEqual(['rule_warn_return']);
	});
	it('does not warn about an archived rule', () => {
		expect(
			ruleWarnings(rule({ archived: true }), [template(REMINDER, 'Relleno', true)], services)
		).toEqual([]);
	});
});

describe('sortRules', () => {
	it('puts before rules first and orders each kind by days', () => {
		const sorted = sortRules([
			rule({ id: 'c', offset_days: 30 }),
			rule({ id: 'b', kind: 'before_appointment', offset_days: 2 }),
			rule({ id: 'd', offset_days: 7 }),
			rule({ id: 'a', kind: 'before_appointment', offset_days: 0 })
		]);
		expect(sorted.map((r) => r.id)).toEqual(['a', 'b', 'd', 'c']);
	});
});
