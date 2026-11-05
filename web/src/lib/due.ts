// The pure rules of the "Para enviar" list (design doc item 74). The server computes the list;
// the phone only hides, from a cached copy, what the server would no longer return.
import type { Appointment, DueItem } from './api/schemas';

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

/** The server's `late` flag splits the list; the order inside each group is the server's. */
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
