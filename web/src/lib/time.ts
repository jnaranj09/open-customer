import { t, type MessageKey } from './i18n';

// A day is a calendar date string, `YYYY-MM-DD`. Day arithmetic runs on UTC noon, so neither the
// browser zone nor a DST change can move it. Only `todayIn`, `formatTime` and `zonedToUtc` look
// at a zone, and they take the business zone as an argument.

const DAY = /^(\d{4})-(\d{2})-(\d{2})$/;
const TIME = /^(\d{2}):(\d{2})$/;

type Ymd = { y: number; m: number; d: number };

function parseDay(day: string): Ymd {
	const match = DAY.exec(day);
	if (!match) {
		throw new RangeError('time: a day must be YYYY-MM-DD');
	}
	return { y: Number(match[1]), m: Number(match[2]), d: Number(match[3]) };
}

const pad = (n: number): string => String(n).padStart(2, '0');

function noon({ y, m, d }: Ymd, plusDays = 0): Date {
	return new Date(Date.UTC(y, m - 1, d + plusDays, 12));
}

const dayOf = (date: Date): string =>
	`${String(date.getUTCFullYear()).padStart(4, '0')}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`;

/** The wall-clock fields of an instant in a zone, read as numbers. */
function wallParts(instant: Date, zone: string): Record<string, number> {
	const parts = new Intl.DateTimeFormat('en-US', {
		timeZone: zone,
		hourCycle: 'h23',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit'
	}).formatToParts(instant);
	const out: Record<string, number> = {};
	for (const p of parts) {
		if (p.type !== 'literal') {
			out[p.type] = Number(p.value);
		}
	}
	return out;
}

/** The zone's offset from UTC at an instant, in ms (positive east of Greenwich). */
function offsetMs(instant: Date, zone: string): number {
	const w = wallParts(instant, zone);
	const asUtc = Date.UTC(
		w['year'] ?? 0,
		(w['month'] ?? 1) - 1,
		w['day'],
		w['hour'],
		w['minute'],
		w['second']
	);
	return asUtc - Math.floor(instant.getTime() / 1000) * 1000;
}

/** The real clock; the one place `new Date()` is read, so everything else takes `now` as an argument. */
export const systemNow = (): Date => new Date();

/** The business day (`YYYY-MM-DD`) in `zone` at `now`. */
export function todayIn(zone: string, now: Date): string {
	const w = wallParts(now, zone);
	return `${String(w['year']).padStart(4, '0')}-${pad(w['month'] ?? 0)}-${pad(w['day'] ?? 0)}`;
}

/** The day `n` calendar days after `day` (`n` may be negative). */
export function addDays(day: string, n: number): string {
	return dayOf(noon(parseDay(day), n));
}

/** The Monday of the week that holds `day`. */
export function weekStart(day: string): string {
	const sinceMonday = (noon(parseDay(day)).getUTCDay() + 6) % 7;
	return addDays(day, -sinceMonday);
}

/** The wall time `HH:MM` of an RFC 3339 instant in `zone`. */
export function formatTime(iso: string, zone: string): string {
	const w = wallParts(new Date(iso), zone);
	return `${pad(w['hour'] ?? 0)}:${pad(w['minute'] ?? 0)}`;
}

const WEEKDAYS: readonly MessageKey[] = [
	'weekday_0',
	'weekday_1',
	'weekday_2',
	'weekday_3',
	'weekday_4',
	'weekday_5',
	'weekday_6'
];

const MONTHS: readonly MessageKey[] = [
	'month_1',
	'month_2',
	'month_3',
	'month_4',
	'month_5',
	'month_6',
	'month_7',
	'month_8',
	'month_9',
	'month_10',
	'month_11',
	'month_12'
];

/** `miércoles 4 de noviembre`; the names come from es.json. */
export function formatDayLong(day: string): string {
	const ymd = parseDay(day);
	const weekday = WEEKDAYS[noon(ymd).getUTCDay()];
	const month = MONTHS[ymd.m - 1];
	if (weekday === undefined || month === undefined) {
		throw new RangeError('time: a day must be a real date');
	}
	return t('date_long', { weekday: t(weekday), day: ymd.d, month: t(month) });
}

/** `mié`: the first three letters of the weekday name. */
export function formatWeekdayShort(day: string): string {
	const weekday = WEEKDAYS[noon(parseDay(day)).getUTCDay()];
	if (weekday === undefined) {
		throw new RangeError('time: a day must be a real date');
	}
	return t(weekday).slice(0, 3);
}

/** `4/11`: day and month, not padded. */
export function formatDayShort(day: string): string {
	const { m, d } = parseDay(day);
	return `${String(d)}/${String(m)}`;
}

/** The UTC instant (RFC 3339, `Z`) of a picked local `day` and `HH:MM` in `zone`. In the hour a DST
 *  change skips or repeats, the result is one of the two valid readings, always the same one. */
export function zonedToUtc(day: string, time: string, zone: string): string {
	const match = TIME.exec(time);
	if (!match) {
		throw new RangeError('time: a time must be HH:MM');
	}
	const { y, m, d } = parseDay(day);
	const wall = Date.UTC(y, m - 1, d, Number(match[1]), Number(match[2]));
	const first = wall - offsetMs(new Date(wall), zone);
	const second = wall - offsetMs(new Date(first), zone);
	return new Date(second).toISOString().replace('.000Z', 'Z');
}

/** `10/3` for a day in the year of `today`, `10/3/2025` for any other year: a history that spans
 *  years must not show two days that look the same. */
export function formatDayWithYear(day: string, today: string): string {
	const { y } = parseDay(day);
	return y === parseDay(today).y ? formatDayShort(day) : `${formatDayShort(day)}/${String(y)}`;
}
