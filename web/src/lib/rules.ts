// The pure parts of Ajustes → Recordatorios (design doc item 77, "Reminder rules and the 'done'
// rule"; general rules and the handover: reminders-and-replies design): how a rule reads as a
// sentence, when a change asks "¿Incluir atrasados?", the form to a request, the patch of what
// changed, and the warnings for archived parts.
import type { Rule, Service, Template } from './api/schemas';
import type { components } from './api/schema';
import { t, type MessageKey } from './i18n';
import { shortDay } from './time';

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

/** True when saving `after` can make new items appear: a new rule, a changed offset, a changed
 *  effective return service (setting it to null when it was the rule's own service changes
 *  nothing), a restore, or a new template for a rule whose old template is in
 *  `archivedTemplates`. */
function addsItems(
	before: Rule | null,
	after: RuleInput | RulePatch,
	archivedTemplates: ReadonlySet<string>
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

/** What saving `after` does about overdue items. `ask`: a come-back change that can add items asks
 *  "¿Incluir atrasados?" first. `include`: the same change on a "before" rule asks
 *  nothing and sends `include_overdue: true`, because its items hide at the appointment's start, so
 *  the only ones "Incluir atrasados" adds are for appointments still ahead whose due day just
 *  passed (a new 2-day rule and tomorrow's visit); "Solo de hoy en adelante" would drop those.
 *  `none`: nothing new can appear; the server ignores the flag. */
export function overdueChoice(
	before: Rule | null,
	after: RuleInput | RulePatch,
	archivedTemplates: ReadonlySet<string> = new Set()
): 'ask' | 'include' | 'none' {
	if (!addsItems(before, after, archivedTemplates)) {
		return 'none';
	}
	const kind = before?.kind ?? ('kind' in after ? after.kind : undefined);
	return kind === 'before_appointment' ? 'include' : 'ask';
}

/** The service picker's value for a general rule: a come-back rule for every service. */
export const ALL_SERVICES = 'all';

/** What the rule form holds. `returnId` and `serviceId` are empty strings when not chosen;
 *  `serviceId` is `ALL_SERVICES` for a general rule. */
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
	if (!before && form.serviceId !== ALL_SERVICES) {
		body.service_id = form.serviceId;
		if (form.returnId !== '') {
			body.return_service_id = form.returnId;
		}
	}
	return { ok: true, body };
}

/** The rule preview's body for a valid come-back form, `ruleId` naming the saved rule it edits;
 *  null for a "before" rule, which has no preview. */
export function previewInput(
	body: RuleInput,
	ruleId: string | undefined
): Schemas['RulePreviewInput'] | null {
	if (body.kind !== 'come_back') {
		return null;
	}
	return {
		...(ruleId === undefined ? {} : { rule_id: ruleId }),
		service_id: body.service_id ?? null,
		return_service_id: body.return_service_id ?? null,
		offset_days: body.offset_days,
		template_id: body.template_id
	};
}

/** "Hoy: 4 clientas · próxima: Carla, 10 nov", or only the count when nothing is ahead. */
export function rulePreviewText(preview: Schemas['RulePreviewResult']): string {
	const count = preview.today_count;
	const next = preview.next;
	if (next === null) {
		return count === 1 ? t('rule_preview_none_one') : t('rule_preview_none', { count });
	}
	const vars = { name: next.client_name, date: shortDay(next.due_date) };
	return count === 1 ? t('rule_preview_one', vars) : t('rule_preview', { count, ...vars });
}

/** The fields of `body` that differ from the saved rule. A come-back rule that goes back to its own
 *  service sends `return_service_id: null`; one whose effective return service is the same sends
 *  none, and a general rule never sends one. */
export function rulePatch(saved: Rule, body: RuleInput): RulePatch {
	const next = body.return_service_id ?? saved.service_id;
	return {
		...(body.offset_days === saved.offset_days ? {} : { offset_days: body.offset_days }),
		...(body.template_id === saved.template_id ? {} : { template_id: body.template_id }),
		...(saved.kind === 'before_appointment' || isGeneral(saved) || next === effectiveReturn(saved)
			? {}
			: { return_service_id: body.return_service_id ?? null })
	};
}

const nameOf = (list: readonly { id: string; name: string }[], id: string | null): string =>
	list.find((row) => row.id === id)?.name ?? '…';

/** A general rule: a come-back rule for every service (no service). */
export function isGeneral(rule: Pick<Rule, 'kind' | 'service_id'>): boolean {
	return rule.kind === 'come_back' && rule.service_id === null;
}

/** "Enviar Relleno 21 días después de Acrílico, para agendar Relleno"; a general rule: "Enviar
 *  Volver 30 días después de la última visita". */
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
	if (isGeneral(rule)) {
		return t('rule_sentence_general', { template, offset });
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

/** The rules in the order she reads them: "before" rules, then come-back rules for one service, then
 *  general rules; each group by days, then by id. */
export function sortRules(rules: readonly Rule[]): Rule[] {
	const rank = (rule: Rule): number =>
		rule.kind === 'before_appointment' ? 0 : isGeneral(rule) ? 2 : 1;
	return [...rules].sort(
		(a, b) => rank(a) - rank(b) || a.offset_days - b.offset_days || a.id.localeCompare(b.id)
	);
}

/** A change that can stop or restart specific rules (reminders-and-replies design, "Handover"). */
export type HandoverChange =
	| { kind: 'rule_archive'; id: string }
	| { kind: 'template_archive' | 'template_restore'; id: string }
	| { kind: 'service_archive' | 'service_restore'; id: string };

type Parts = { templates: readonly Template[]; services: readonly Service[] };

/** Whether `rule` can make items with these templates and services: it, its template, its service
 *  and its effective return service are active. */
function canSend(rule: Rule, parts: Parts): boolean {
	const active = (list: readonly { id: string; archived: boolean }[], id: string | null) =>
		id === null || list.find((row) => row.id === id)?.archived === false;
	return (
		!rule.archived &&
		active(parts.templates, rule.template_id) &&
		active(parts.services, rule.service_id) &&
		active(parts.services, effectiveReturn(rule))
	);
}

/** The rules and parts as they will be once `change` is saved. */
function afterChange(change: HandoverChange, rules: readonly Rule[], parts: Parts) {
	const flip = <T extends { id: string; archived: boolean }>(
		list: readonly T[],
		archived: boolean
	) => list.map((row) => (row.id === change.id ? { ...row, archived } : row));
	switch (change.kind) {
		case 'rule_archive':
			return { rules: flip(rules, true), parts };
		case 'template_archive':
		case 'template_restore':
			return {
				rules,
				parts: { ...parts, templates: flip(parts.templates, change.kind === 'template_archive') }
			};
		case 'service_archive':
		case 'service_restore':
			return {
				rules,
				parts: { ...parts, services: flip(parts.services, change.kind === 'service_archive') }
			};
	}
}

/** Whether `change` asks the handover question ("Solo de hoy en adelante" / "Incluir atrasados"):
 *  a general rule can send once it is saved, and it stops or restarts at least one rule for one
 *  service. Otherwise the web sends no answer, which means "from today". From the cached data. */
export function handoverAsks(
	change: HandoverChange,
	rules: readonly Rule[],
	templates: readonly Template[],
	services: readonly Service[]
): boolean {
	const now: Parts = { templates, services };
	const next = afterChange(change, rules, now);
	if (!next.rules.some((rule) => isGeneral(rule) && canSend(rule, next.parts))) {
		return false;
	}
	return rules.some(
		(rule, i) =>
			rule.kind === 'come_back' &&
			!isGeneral(rule) &&
			canSend(rule, now) !== canSend(next.rules[i] ?? rule, next.parts)
	);
}
