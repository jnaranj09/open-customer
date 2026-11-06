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
 *  minutes since local midnight (clipped to the day). `continued` is true for the tail of one that
 *  started on an earlier day (it shows from 00:00). */
export type Placed = {
	id: string;
	col: number;
	cols: number;
	startMin: number;
	endMin: number;
	continued: boolean;
};

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

/** The minutes a box's 44 px tap height (`min-h-tap`) covers in the grid (TimeGrid: one hour is
 *  `--hour: 4rem`, 64 px), rounded up. A shorter appointment is drawn this tall. */
const TAP_MINUTES = Math.ceil((44 / 64) * 60);

/** Places the day's appointments in the time grid. Cancelled ones are left out; "no vino" ones stay
 *  (the view greys them). Columns follow the drawn span: a box is at least `TAP_MINUTES` tall, so
 *  an item ends, for the layout, at `max(end, start + TAP_MINUTES)`. Items whose drawn spans
 *  overlap share the width in side-by-side columns, so a short box never covers the next one;
 *  back-to-back items of at least `TAP_MINUTES` do not overlap. */
export function layoutDay(items: AgendaItem[], zone: string, day: string): Placed[] {
	const dayStart = Date.parse(zonedToUtc(day, '00:00', zone));
	const dayEnd = Date.parse(zonedToUtc(addDays(day, 1), '00:00', zone));
	const spans = items
		.filter((i) => i.status !== 'cancelled')
		.map((i) => ({
			id: i.id,
			startMin: minuteOn(i.starts_at, zone, dayStart, dayEnd),
			endMin: minuteOn(i.ends_at, zone, dayStart, dayEnd),
			continued: Date.parse(i.starts_at) < dayStart
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
		const drawnEnd = Math.max(s.endMin, s.startMin + TAP_MINUTES);
		let col = columnEnds.findIndex((end) => end <= s.startMin);
		if (col === -1) {
			col = columnEnds.length;
		}
		columnEnds[col] = drawnEnd;
		groupEnd = Math.max(groupEnd, drawnEnd);
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

/** The business opening hour: there is no business-hours setting, so the grid assumes 08:00. */
const OPENING_HOUR = 8;

/** The hour a grid scrolls to when it opens: the one that holds the earliest start, or the opening
 *  hour when nothing starts on the shown days. The tail of an appointment that crossed midnight
 *  (`continued`) does not count: it would open the whole week at 00:00. The grid always holds the
 *  whole day; this is only where it starts. */
export function scrollHour(placed: Placed[]): number {
	const starts = placed.filter((p) => !p.continued).map((p) => Math.floor(p.startMin / 60));
	return starts.length === 0 ? OPENING_HOUR : Math.min(...starts);
}

/** What to remember of a day she picked: nothing (`null`, follow the business day) when it is
 *  today, so a resume after midnight moves to the new day; any other day as she chose it. */
export function pickedValue(picked: string, today: string): string | null {
	return picked === today ? null : picked;
}
