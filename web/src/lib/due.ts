// The pure rules of the "Para enviar" list (design doc item 74). The server computes the list;
// the phone only hides, from a cached copy, what the server would no longer return.
import type { Api } from './api/client';
import type { Result } from './api/errors';
import type { Appointment, DueItem } from './api/schemas';
import { t } from './i18n';
import { daysBetween, formatTime, shortDay, todayIn } from './time';

/** The list to show. Online it is the server's list as sent. Offline, a "before" item whose
 *  `anchor_at` (the appointment's start) has passed is hidden, because the server would no longer
 *  return it; this is the only thing the phone computes about the list. */
export function visibleDue(items: DueItem[], now: Date, offline: boolean): DueItem[] {
	if (!offline) {
		return items;
	}
	return items.filter(
		(i) =>
			!(
				i.kind === 'before_appointment' &&
				i.anchor_at !== null &&
				Date.parse(i.anchor_at) <= now.getTime()
			)
	);
}

/** The due list every screen shows and counts: "Para enviar", Hoy's card and rail, the sidebar
 *  count and the app badge. It is `visibleDue`, minus the items the server has already logged
 *  (`logged`) that the cached list still holds because no refresh has dropped them yet. */
export function shownDue(
	items: DueItem[],
	at: { now: Date; offline: boolean; logged: readonly string[] }
): DueItem[] {
	return visibleDue(items, at.now, at.offline).filter((i) => !at.logged.includes(i.id));
}

/** The server's `late` flag splits the list; the order inside each group is the server's (the
 *  cache keeps it: `position` on each cached due row, lib/cache/db.ts). */
export function groupDue(items: DueItem[]): { late: DueItem[]; today: DueItem[] } {
	return { late: items.filter((i) => i.late), today: items.filter((i) => !i.late) };
}

/** The item after `id` in `items`, or null when `id` is last or not there. */
export function nextAfter(items: DueItem[], id: string): DueItem | null {
	const at = items.findIndex((i) => i.id === id);
	return at === -1 ? null : (items[at + 1] ?? null);
}

/** The `service_ids` for "Agregar a la cita": the appointment's lines, then the return service. */
export function addReturnService(appt: Appointment, returnServiceId: string): string[] {
	return [...appt.services.map((s) => s.service_id), returnServiceId];
}

/** "Agregar a la cita": reads the appointment as it is now on the server, then sends its
 *  `service_ids` plus the return service. `service_ids` replaces the whole list, so it must not be
 *  built from a cached copy that another device may have changed. */
export async function addToBooking(
	api: Pick<Api, 'getAppointment' | 'updateAppointment'>,
	prompt: NonNullable<DueItem['prompt']>,
	allowOverlap: boolean
): Promise<Result<Appointment>> {
	const current = await api.getAppointment(prompt.appointment_id);
	if (!current.ok) {
		return current;
	}
	return api.updateAppointment(prompt.appointment_id, {
		service_ids: addReturnService(current.data, prompt.return_service_id),
		...(allowOverlap ? { allow_overlap: true } : {})
	});
}

/** The days left when a label turns into a warning. */
const WARN_DAYS = 7;
/** How many service names a label spells out before "+n". */
const NAMED_SERVICES = 2;

/** The label of a due item (reminders-and-replies design, "Row label"), from the item's fields only:
 *  the source visit, its services, "General N días" for a general rule, "Nº mensaje" from the 2nd,
 *  and, online only (offline the cached list may be a day old), the days left. `warning` when a
 *  week or less is left. "Before" items show the visit's time and no days left. */
export function dueLabel(
	item: DueItem,
	today: string,
	zone: string,
	online: boolean
): { text: string; warning: boolean } {
	const date = shortDay(todayIn(zone, new Date(item.source_starts_at)));
	const before = item.kind === 'before_appointment';
	const named = item.source_services.slice(0, NAMED_SERVICES).join(', ');
	const more = item.source_services.length - NAMED_SERVICES;
	const parts = [
		before
			? t('due_label_visit_time', { date, time: formatTime(item.source_starts_at, zone) })
			: t('due_label_visit', { date }),
		...(named === '' ? [] : [more > 0 ? `${named} ${t('due_label_more', { n: more })}` : named])
	];
	if (before) {
		return { text: parts.join(' · '), warning: false };
	}
	if (item.general) {
		parts.push(t('due_label_general', { n: item.offset_days }));
	}
	if (item.message_number >= 2) {
		parts.push(t('due_label_number', { n: item.message_number }));
	}
	const left = daysBetween(today, item.shown_until);
	if (online) {
		parts.push(
			left >= 2
				? t('due_label_left_other', { n: left })
				: left === 1
					? t('due_label_left_one')
					: t('due_label_last_day')
		);
	}
	return { text: parts.join(' · '), warning: online && left <= WARN_DAYS };
}
