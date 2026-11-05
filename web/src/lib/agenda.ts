// The pure part of Hoy (design doc item 70, 72): where appointments sit in a day's time grid, and
// which days the cache can answer.
import type { components } from './api/schema';
import { addDays, formatTime, weekStart, zonedToUtc } from './time';

type AppointmentStatus = components['schemas']['AppointmentStatus'];

/** What the grid needs of an appointment. */
export type AgendaItem = {
	id: string;
	starts_at: string;
	ends_at: string;
	status: AppointmentStatus;
};

/** One appointment in a day's grid: the column it takes among `cols` side by side, and its span in
 *  minutes since local midnight (clipped to the day). */
export type Placed = { id: string; col: number; cols: number; startMin: number; endMin: number };

export type AgendaView = 'day' | 'week';

const DAY_MIN = 24 * 60;

/** Minutes since local midnight of `iso` on `day`; 0 before the day and 24:00 after it. */
function minuteOn(iso: string, zone: string, dayStart: number, dayEnd: number): number {
	const at = Date.parse(iso);
	if (at <= dayStart) return 0;
	if (at >= dayEnd) return DAY_MIN;
	const [h, m] = formatTime(iso, zone).split(':');
	return Number(h) * 60 + Number(m);
}

/** Places the day's appointments in the time grid. Cancelled ones are left out; "no vino" ones stay
 *  (the view greys them). Overlapping ones (`a.start < b.end && a.end > b.start`) share the width
 *  in side-by-side columns; back-to-back ones do not overlap. */
export function layoutDay(items: AgendaItem[], zone: string, day: string): Placed[] {
	const dayStart = Date.parse(zonedToUtc(day, '00:00', zone));
	const dayEnd = Date.parse(zonedToUtc(addDays(day, 1), '00:00', zone));
	const spans = items
		.filter((i) => i.status !== 'cancelled')
		.map((i) => ({
			id: i.id,
			startMin: minuteOn(i.starts_at, zone, dayStart, dayEnd),
			endMin: minuteOn(i.ends_at, zone, dayStart, dayEnd)
		}))
		.filter((s) => s.endMin > s.startMin)
		.sort((a, b) => a.startMin - b.startMin || a.endMin - b.endMin);

	const placed: Placed[] = [];
	let group: Placed[] = [];
	let groupEnd = 0;
	// The end of the last item in each column of the open group.
	let columnEnds: number[] = [];
	const close = (): void => {
		for (const p of group) {
			placed.push({ ...p, cols: columnEnds.length });
		}
		group = [];
		columnEnds = [];
	};
	for (const s of spans) {
		if (group.length > 0 && s.startMin >= groupEnd) {
			close();
		}
		let col = columnEnds.findIndex((end) => end <= s.startMin);
		if (col === -1) {
			col = columnEnds.length;
		}
		columnEnds[col] = s.endMin;
		groupEnd = Math.max(groupEnd, s.endMin);
		group.push({ ...s, col, cols: 0 });
	}
	close();
	return placed;
}

/** True when the cache (the `/bootstrap` window) holds `day`. */
export function inWindow(day: string, win: { from: string; to: string }): boolean {
	return day >= win.from && day <= win.to;
}

/** The days a view shows: the day itself, or its Monday-to-Sunday week. */
export function viewRange(view: AgendaView, day: string): { from: string; to: string } {
	if (view === 'day') {
		return { from: day, to: day };
	}
	const from = weekStart(day);
	return { from, to: addDays(from, 6) };
}

const DEFAULT_FROM_HOUR = 8;
const DEFAULT_TO_HOUR = 19;

/** The hours a grid shows: 08:00 to 19:00, widened to the hour that holds the earliest start and
 *  the latest end. */
export function hourSpan(placed: Placed[]): { from: number; to: number } {
	const starts = placed.map((p) => Math.floor(p.startMin / 60));
	const ends = placed.map((p) => Math.ceil(p.endMin / 60));
	return {
		from: Math.min(DEFAULT_FROM_HOUR, ...starts),
		to: Math.max(DEFAULT_TO_HOUR, ...ends)
	};
}
