// The pure part of "Agendar próxima cita" (next-appointment design, sections 1 to 3): from one
// appointment and the cached come-back rules, the dates her rules give for the next visit and the
// services each one books. It reads two fields of a rule (the days and the return service) and
// computes nothing about what is due: the due list stays the server's.
import type { Appointment, Rule, Service, Template } from './api/schemas';
import { activeServiceIds, linesOf, type Line } from './booking';
import { t } from './i18n';
import { effectiveReturn, ruleCanSend } from './rules';
import { addDays, shortDay, todayIn } from './time';

/** One date she can pick: `days` after the visit, on `day` (`YYYY-MM-DD`), booking `lines`. */
export type Choice = { days: number; day: string; lines: Line[] };
/** The date choices, rising, and the services for a day she types herself ("Otra fecha"). */
export type NextVisit = { choices: Choice[]; otherDate: Line[] };

const byDaysThenId = (a: Rule, b: Rule): number =>
	a.offset_days - b.offset_days || a.id.localeCompare(b.id);

/** The next visit after `appointment`. A come-back rule counts when it can send and is for a
 *  service on the appointment; the general rules count only when no such rule does. The days run
 *  from the local day the visit ends, and a choice before `today` is left out. Each service is
 *  replaced by what its rule says comes next: its rule of the picked days, else its rule with the
 *  fewest days (the lower id on a tie); a service with no rule is copied. */
export function nextVisit(
	appointment: Pick<Appointment, 'ends_at' | 'services'>,
	rules: readonly Rule[],
	templates: readonly Template[],
	services: readonly Service[],
	zone: string,
	today: string
): NextVisit {
	const visit = linesOf(appointment.services);
	const onVisit = new Set(visit.map((l) => l.service_id));
	const sending = rules.filter(
		(rule) => rule.kind === 'come_back' && ruleCanSend(rule, { templates, services })
	);
	const specific = sending
		.filter((rule) => rule.service_id !== null && onVisit.has(rule.service_id))
		.sort(byDaysThenId);
	const counted = specific.length > 0 ? specific : sending.filter((r) => r.service_id === null);
	const bookable = activeServiceIds([...services]);

	const ruleFor = (serviceId: string, days: number | null): Rule | undefined => {
		const own = specific.filter((rule) => rule.service_id === serviceId);
		return own.find((rule) => rule.offset_days === days) ?? own[0];
	};
	const linesFor = (days: number | null): Line[] => {
		const out: Line[] = [];
		for (const l of visit) {
			const rule = ruleFor(l.service_id, days);
			const target = (rule === undefined ? null : effectiveReturn(rule)) ?? l.service_id;
			if (!bookable.has(target)) {
				continue;
			}
			const same = out.find((o) => o.service_id === target);
			if (same === undefined) {
				out.push({ service_id: target, count: l.count });
			} else {
				same.count = Math.max(same.count, l.count);
			}
		}
		return out;
	};

	const base = todayIn(zone, new Date(appointment.ends_at));
	const offsets = [...new Set(counted.map((rule) => rule.offset_days))].sort((a, b) => a - b);
	return {
		choices: offsets
			.map((days) => ({ days, day: addDays(base, days), lines: linesFor(days) }))
			.filter((choice) => choice.day >= today),
		otherDate: linesFor(null)
	};
}

/** "En 21 días · 3 dic"; "En 1 día · 4 dic" for one day. */
export function choiceTitle(choice: Choice): string {
	const date = shortDay(choice.day);
	return choice.days === 1
		? t('next_choice_one', { date })
		: t('next_choice_other', { n: choice.days, date });
}

/** "Relleno ×2, Pedicure"; "Sin servicios" for no line. A service not in `services` is left out. */
export function linesText(lines: readonly Line[], services: readonly Service[]): string {
	const names = lines.flatMap((l) => {
		const name = services.find((s) => s.id === l.service_id)?.name;
		if (name === undefined) {
			return [];
		}
		return [l.count > 1 ? `${name} ${t('appointment_service_count', { count: l.count })}` : name];
	});
	return names.length === 0 ? t('appointment_no_services') : names.join(', ');
}

/** Whether "Agendar próxima cita" shows: the visit is scheduled or confirmed (a cancelled one or
 *  a no-show is not a visit to count from), and the client is in `clients` (the cache lists only
 *  the active ones, and an archived client cannot be booked). */
export function canBookNext(
	appointment: Pick<Appointment, 'status' | 'client_id'>,
	clients: readonly { id: string }[]
): boolean {
	return (
		(appointment.status === 'scheduled' || appointment.status === 'confirmed') &&
		clients.some((c) => c.id === appointment.client_id)
	);
}
