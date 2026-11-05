import { describe, expect, it } from 'vitest';
import {
	addDays,
	formatDayLong,
	formatDayShort,
	formatWeekdayShort,
	formatTime,
	todayIn,
	weekStart,
	zonedToUtc
} from './time';

describe('todayIn', () => {
	const now = new Date('2026-11-05T04:30:00Z');

	it('uses the business zone, not the browser zone (Costa Rica is still on the 4th)', () => {
		expect(todayIn('America/Costa_Rica', now)).toBe('2026-11-04');
	});

	it('uses the business zone, not the browser zone (Madrid is already on the 5th)', () => {
		expect(todayIn('Europe/Madrid', now)).toBe('2026-11-05');
	});
});

describe('addDays', () => {
	it('adds days across a month end', () => {
		expect(addDays('2026-11-29', 3)).toBe('2026-12-02');
	});

	it('subtracts days across a year start', () => {
		expect(addDays('2026-01-02', -3)).toBe('2025-12-30');
	});

	it('adds across a leap day', () => {
		expect(addDays('2028-02-28', 2)).toBe('2028-03-01');
	});

	it('rejects a malformed day', () => {
		expect(() => addDays('4/11/2026', 1)).toThrow(RangeError);
	});
});

describe('weekStart', () => {
	it('returns Monday for a Sunday', () => {
		expect(weekStart('2026-11-08')).toBe('2026-11-02');
	});

	it('returns the same day for a Monday', () => {
		expect(weekStart('2026-11-02')).toBe('2026-11-02');
	});

	it('returns Monday for a Wednesday', () => {
		expect(weekStart('2026-11-04')).toBe('2026-11-02');
	});
});

describe('formatTime', () => {
	it('shows the wall time in the zone', () => {
		expect(formatTime('2026-11-04T21:00:00Z', 'America/Costa_Rica')).toBe('15:00');
	});

	it('shows midnight as 00:00', () => {
		expect(formatTime('2026-11-04T06:05:00Z', 'America/Costa_Rica')).toBe('00:05');
	});
});

describe('formatDayLong', () => {
	it('names the weekday and the month in Spanish', () => {
		expect(formatDayLong('2026-11-04')).toBe('miércoles 4 de noviembre');
	});
});

describe('formatWeekdayShort', () => {
	it('gives the first three letters of the weekday', () => {
		expect(formatWeekdayShort('2026-11-04')).toBe('mié');
	});

	it('starts the week on lunes for a Monday', () => {
		expect(formatWeekdayShort('2026-11-02')).toBe('lun');
	});
});

describe('formatDayShort', () => {
	it('shows day/month without padding the day', () => {
		expect(formatDayShort('2026-11-04')).toBe('4/11');
	});

	it('shows a two-digit day', () => {
		expect(formatDayShort('2026-11-12')).toBe('12/11');
	});
});

describe('zonedToUtc', () => {
	it('converts a Costa Rica wall time (UTC-6, no DST)', () => {
		expect(zonedToUtc('2026-11-04', '15:00', 'America/Costa_Rica')).toBe('2026-11-04T21:00:00Z');
	});

	it('uses summer time in Madrid before the change', () => {
		expect(zonedToUtc('2026-10-24', '10:00', 'Europe/Madrid')).toBe('2026-10-24T08:00:00Z');
	});

	it('uses winter time in Madrid across the DST change', () => {
		expect(zonedToUtc('2026-10-25', '10:00', 'Europe/Madrid')).toBe('2026-10-25T09:00:00Z');
	});

	it('settles on the right offset just after a spring-forward change', () => {
		expect(zonedToUtc('2026-03-08', '03:30', 'America/New_York')).toBe('2026-03-08T07:30:00Z');
	});

	it('round trips through formatTime', () => {
		expect(formatTime(zonedToUtc('2026-03-29', '12:30', 'Europe/Madrid'), 'Europe/Madrid')).toBe(
			'12:30'
		);
	});

	it('rejects a malformed time', () => {
		expect(() => zonedToUtc('2026-11-04', '3pm', 'Europe/Madrid')).toThrow(RangeError);
	});
});
