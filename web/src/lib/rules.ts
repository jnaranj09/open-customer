// The pure parts of Ajustes → Recordatorios (design doc item 77, "Reminder rules and the 'done'
// rule"): how a rule reads as a sentence, when a change asks "¿Incluir recordatorios atrasados?",
// the form to a request, the patch of what changed, and the warnings for archived parts.
import type { Rule, Service, Template } from './api/schemas';
import type { components } from './api/schema';
import { t, type MessageKey } from './i18n';

type Schemas = components['schemas'];

export type RuleKind = Rule['kind'];
export type RuleInput = Schemas['RuleInput'];
export type RulePatch = Schemas['RulePatch'];

/** "1 día antes", "el mismo día" (0), "21 días después". */
export function offsetText(kind: RuleKind, days: number): string {
	if (days === 0) {
		return t('offset_same_day');
	}
	const side = kind === 'before_appointment' ? 'before' : 'after';
	return days === 1 ? t(`offset_${side}_one`) : t(`offset_${side}_other`, { n: days });
}

/** The service a come-back rule asks her to book: its return service, or its own when that is empty. */
function effectiveReturn(rule: Pick<Rule, 'service_id' | 'return_service_id'>): string | null {
	return rule.return_service_id ?? rule.service_id;
}

/** True when saving `after` can make new items appear, so the app asks "¿Incluir recordatorios
 *  atrasados?" first: a new rule, a changed offset, a changed effective return service (setting it
 *  to null when it was the rule's own service changes nothing), a restore, or a new template for a
 *  rule whose old template is in `archivedTemplates`. */
export function asksOverdue(
	before: Rule | null,
	after: RuleInput | RulePatch,
	archivedTemplates: ReadonlySet<string> = new Set()
): boolean {
	if (before === null) {
		return true;
	}
	if (after.offset_days !== undefined && after.offset_days !== before.offset_days) {
		return true;
	}
	if (
		after.template_id !== undefined &&
		after.template_id !== before.template_id &&
		archivedTemplates.has(before.template_id)
	) {
		return true;
	}
	if ('archived' in after && !after.archived && before.archived) {
		return true;
	}
	if (after.return_service_id !== undefined) {
		return (after.return_service_id ?? before.service_id) !== effectiveReturn(before);
	}
	return false;
}

/** What the rule form holds. `returnId` and `serviceId` are empty strings when not chosen. */
export type RuleForm = {
	kind: RuleKind;
	templateId: string;
	days: string;
	serviceId: string;
	returnId: string;
};

export type RuleField = 'template' | 'days' | 'service';

const MAX_DAYS = 365;

/** The request for the form, or the message key of each field that is not valid. */
export function ruleRequest(
	form: RuleForm
): { ok: true; body: RuleInput } | { ok: false; errors: Partial<Record<RuleField, MessageKey>> } {
	const errors: Partial<Record<RuleField, MessageKey>> = {};
	const before = form.kind === 'before_appointment';
	if (form.templateId === '') {
		errors.template = 'rule_template_required';
	}
	const days = /^\d+$/.test(form.days.trim()) ? Number(form.days.trim()) : -1;
	if (days < (before ? 0 : 1) || days > MAX_DAYS) {
		errors.days = before ? 'rule_days_invalid_before' : 'rule_days_invalid_come_back';
	}
	if (!before && form.serviceId === '') {
		errors.service = 'rule_service_required';
	}
	if (Object.keys(errors).length > 0) {
		return { ok: false, errors };
	}
	const body: RuleInput = { kind: form.kind, template_id: form.templateId, offset_days: days };
	if (!before) {
		body.service_id = form.serviceId;
		if (form.returnId !== '') {
			body.return_service_id = form.returnId;
		}
	}
	return { ok: true, body };
}

/** The fields of `body` that differ from the saved rule. A come-back rule that goes back to its own
 *  service sends `return_service_id: null`; one whose effective return service is the same sends none. */
export function rulePatch(saved: Rule, body: RuleInput): RulePatch {
	const next = body.return_service_id ?? saved.service_id;
	return {
		...(body.offset_days === saved.offset_days ? {} : { offset_days: body.offset_days }),
		...(body.template_id === saved.template_id ? {} : { template_id: body.template_id }),
		...(saved.kind === 'before_appointment' || next === effectiveReturn(saved)
			? {}
			: { return_service_id: body.return_service_id ?? null })
	};
}

const nameOf = (list: readonly { id: string; name: string }[], id: string | null): string =>
	list.find((row) => row.id === id)?.name ?? '…';

/** "Enviar Relleno 21 días después de Acrílico, para agendar Relleno". */
export function ruleSentence(
	rule: Rule,
	templates: readonly Template[],
	services: readonly Service[]
): string {
	const template = nameOf(templates, rule.template_id);
	const offset = offsetText(rule.kind, rule.offset_days);
	if (rule.kind === 'before_appointment') {
		return t('rule_sentence_before', { template, offset });
	}
	return t('rule_sentence_come_back', {
		template,
		offset,
		service: nameOf(services, rule.service_id),
		returnService: nameOf(services, effectiveReturn(rule))
	});
}

/** The archived parts of an active rule, as message keys. An archived rule has none. */
export function ruleWarnings(
	rule: Rule,
	templates: readonly Template[],
	services: readonly Service[]
): MessageKey[] {
	if (rule.archived) {
		return [];
	}
	const archived = (list: readonly { id: string; archived: boolean }[], id: string | null) =>
		list.find((row) => row.id === id)?.archived === true;
	return [
		...(archived(templates, rule.template_id) ? (['rule_warn_template'] as const) : []),
		...(archived(services, rule.service_id) ? (['rule_warn_service'] as const) : []),
		...(rule.return_service_id !== null && archived(services, rule.return_service_id)
			? (['rule_warn_return'] as const)
			: [])
	];
}

/** The rules in the order she reads them: "before" rules first, then each kind by days, then by id. */
export function sortRules(rules: readonly Rule[]): Rule[] {
	const rank = (rule: Rule): number => (rule.kind === 'before_appointment' ? 0 : 1);
	return [...rules].sort(
		(a, b) => rank(a) - rank(b) || a.offset_days - b.offset_days || a.id.localeCompare(b.id)
	);
}
