import { describe, expect, it } from 'vitest';
import { hourSpan, inWindow, layoutDay, pickedValue, viewRange, type AgendaItem } from './agenda';

const ZONE = 'America/Costa_Rica';
const DAY = '2026-11-04';

// Costa Rica is UTC-6 all year, so 09:00 local is 15:00Z.
function item(id: string, start: string, end: string, status: AgendaItem['status'] = 'scheduled') {
	return { id, starts_at: start, ends_at: end, status };
}

describe('layoutDay', () => {
	it('puts three overlapping items in three columns', () => {
		const placed = layoutDay(
			[
				item('a', '2026-11-04T15:00:00Z', '2026-11-04T17:00:00Z'),
				item('b', '2026-11-04T15:30:00Z', '2026-11-04T16:30:00Z'),
				item('c', '2026-11-04T16:00:00Z', '2026-11-04T17:00:00Z')
			],
			ZONE,
			DAY
		);
		expect(placed.map((p) => [p.id, p.col, p.cols])).toEqual([
			['a', 0, 3],
			['b', 1, 3],
			['c', 2, 3]
		]);
	});

	it('reuses a freed column inside one overlap group', () => {
		const placed = layoutDay(
			[
				item('a', '2026-11-04T15:00:00Z', '2026-11-04T16:00:00Z'),
				item('b', '2026-11-04T15:30:00Z', '2026-11-04T17:00:00Z'),
				item('c', '2026-11-04T16:00:00Z', '2026-11-04T16:45:00Z')
			],
			ZONE,
			DAY
		);
		expect(placed.map((p) => [p.id, p.col, p.cols])).toEqual([
			['a', 0, 2],
			['b', 1, 2],
			['c', 0, 2]
		]);
	});

	it('keeps back-to-back items full width', () => {
		const placed = layoutDay(
			[
				item('a', '2026-11-04T15:00:00Z', '2026-11-04T16:00:00Z'),
				item('b', '2026-11-04T16:00:00Z', '2026-11-04T17:00:00Z')
			],
			ZONE,
			DAY
		);
		expect(placed.map((p) => [p.id, p.col, p.cols])).toEqual([
			['a', 0, 1],
			['b', 0, 1]
		]);
	});

	it('hides cancelled and keeps no_show', () => {
		const placed = layoutDay(
			[
				item('gone', '2026-11-04T15:00:00Z', '2026-11-04T16:00:00Z', 'cancelled'),
				item('missed', '2026-11-04T16:00:00Z', '2026-11-04T17:00:00Z', 'no_show')
			],
			ZONE,
			DAY
		);
		expect(placed.map((p) => p.id)).toEqual(['missed']);
	});

	it('does not let a cancelled item push a live one into a column', () => {
		const placed = layoutDay(
			[
				item('gone', '2026-11-04T15:00:00Z', '2026-11-04T16:00:00Z', 'cancelled'),
				item('live', '2026-11-04T15:00:00Z', '2026-11-04T16:00:00Z')
			],
			ZONE,
			DAY
		);
		expect(placed.map((p) => [p.id, p.col, p.cols])).toEqual([['live', 0, 1]]);
	});

	it('gives minutes of the local day', () => {
		const placed = layoutDay(
			[item('a', '2026-11-04T15:30:00Z', '2026-11-04T16:15:00Z')],
			ZONE,
			DAY
		);
		expect(placed.map((p) => [p.startMin, p.endMin])).toEqual([[9 * 60 + 30, 10 * 60 + 15]]);
	});

	it('clips an appointment that crosses midnight to the day', () => {
		const late = item('late', '2026-11-05T05:30:00Z', '2026-11-05T07:00:00Z');
		expect(layoutDay([late], ZONE, DAY).map((p) => [p.startMin, p.endMin])).toEqual([
			[23 * 60 + 30, 24 * 60]
		]);
		expect(layoutDay([late], ZONE, '2026-11-05').map((p) => [p.startMin, p.endMin])).toEqual([
			[0, 60]
		]);
	});

	it('leaves out an item on another day', () => {
		const other = item('o', '2026-11-05T15:00:00Z', '2026-11-05T16:00:00Z');
		expect(layoutDay([other], ZONE, DAY)).toEqual([]);
	});
});

describe('inWindow', () => {
	const win = { from: '2026-09-05', to: '2027-03-04' };

	it('is true on both edges', () => {
		expect([inWindow('2026-09-05', win), inWindow('2027-03-04', win)]).toEqual([true, true]);
	});

	it('is false one day past the window', () => {
		expect(inWindow('2027-03-05', win)).toBe(false);
	});

	it('is false one day before the window', () => {
		expect(inWindow('2026-09-04', win)).toBe(false);
	});
});

describe('viewRange', () => {
	it('is the day itself for the day view', () => {
		expect(viewRange('day', '2026-11-04')).toEqual({ from: '2026-11-04', to: '2026-11-04' });
	});

	it('is Monday to Sunday for the week view', () => {
		expect(viewRange('week', '2026-11-04')).toEqual({ from: '2026-11-02', to: '2026-11-08' });
	});
});

describe('hourSpan', () => {
	it('is 08:00 to 19:00 for an empty day', () => {
		expect(hourSpan([])).toEqual({ from: 8, to: 19 });
	});

	it('widens to an early start and a late end', () => {
		const placed = [
			{ id: 'a', col: 0, cols: 1, startMin: 6 * 60 + 30, endMin: 7 * 60 },
			{ id: 'b', col: 0, cols: 1, startMin: 20 * 60, endMin: 21 * 60 + 15 }
		];
		expect(hourSpan(placed)).toEqual({ from: 6, to: 22 });
	});
});

describe('pickedValue', () => {
	it('stores no day when she picks today, so Hoy follows midnight', () => {
		expect(pickedValue('2026-11-04', '2026-11-04')).toBeNull();
	});

	it('keeps another picked day as she chose it', () => {
		expect(pickedValue('2026-11-07', '2026-11-04')).toBe('2026-11-07');
	});
});
