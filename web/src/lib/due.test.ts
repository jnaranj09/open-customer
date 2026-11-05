import { describe, expect, it } from 'vitest';
import type { Appointment, DueItem } from './api/schemas';
import { addReturnService, groupDue, nextAfter, visibleDue } from './due';

const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const RULE = '7a1c9e20-2b3d-4f5a-8b6c-0d9e8f7a6b5c';
const NOW = new Date('2026-11-04T15:00:00Z');

function item(over: Partial<DueItem>): DueItem {
	return {
		id: 'a',
		rule_id: RULE,
		appointment_id: ID,
		anchor_at: null,
		kind: 'come_back',
		client_id: ID,
		client_name: 'Marta Solano',
		due_date: '2026-11-04',
		late: false,
		text: 'Hola',
		phone_digits: '50688887777',
		wa_link: 'https://wa.me/50688887777',
		mode: 'message',
		prompt: null,
		...over
	};
}

const before = (id: string, anchor: string): DueItem =>
	item({ id, kind: 'before_appointment', anchor_at: anchor });

describe('visibleDue', () => {
	it('hides a cached before item once its anchor_at has passed', () => {
		const items = [before('past', '2026-11-04T14:00:00Z'), before('soon', '2026-11-04T16:00:00Z')];
		expect(visibleDue(items, NOW, true).map((i) => i.id)).toEqual(['soon']);
	});

	it('hides a before item whose anchor_at is exactly now', () => {
		expect(visibleDue([before('now', NOW.toISOString())], NOW, true)).toEqual([]);
	});

	it('keeps a come-back item offline', () => {
		expect(visibleDue([item({ id: 'c' })], NOW, true).map((i) => i.id)).toEqual(['c']);
	});

	it('shows an online list as the server sent it', () => {
		const items = [before('past', '2026-11-04T14:00:00Z')];
		expect(visibleDue(items, NOW, false)).toEqual(items);
	});
});

describe('groupDue', () => {
	it('splits late and today, keeping the order', () => {
		const items = [
			item({ id: 'l1', late: true }),
			item({ id: 't1' }),
			item({ id: 'l2', late: true })
		];
		const { late, today } = groupDue(items);
		expect([late.map((i) => i.id), today.map((i) => i.id)]).toEqual([['l1', 'l2'], ['t1']]);
	});
});

describe('nextAfter', () => {
	const items = [item({ id: 'a' }), item({ id: 'b' }), item({ id: 'c' })];

	it('returns the next item', () => {
		expect(nextAfter(items, 'a')?.id).toBe('b');
	});

	it('returns null at the end', () => {
		expect(nextAfter(items, 'c')).toBeNull();
	});

	it('returns null for an id that is not in the list', () => {
		expect(nextAfter(items, 'zzz')).toBeNull();
	});
});

describe('addReturnService', () => {
	const appt = (serviceIds: string[]): Appointment =>
		({
			id: ID,
			services: serviceIds.map((service_id) => ({
				service_id,
				name: 'x',
				price_cents: null,
				duration_min: 30
			}))
		}) as unknown as Appointment;

	it('appends the return service to the appointment lines', () => {
		expect(addReturnService(appt(['s1', 's2']), 's3')).toEqual(['s1', 's2', 's3']);
	});

	it('keeps a service she booked twice', () => {
		expect(addReturnService(appt(['s1', 's1']), 's3')).toEqual(['s1', 's1', 's3']);
	});
});
