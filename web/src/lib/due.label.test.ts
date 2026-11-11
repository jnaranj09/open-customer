// The row label of a due item (reminders-and-replies design, "Row label"; item 34).
import { describe, expect, it } from 'vitest';
import type { DueItem } from './api/schemas';
import { dueLabel, nextReminderText } from './due';

const ID = '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f';
const ZONE = 'America/Costa_Rica';
const TODAY = '2026-10-31';

function item(over: Partial<DueItem>): DueItem {
	return {
		id: 'a',
		rule_id: ID,
		appointment_id: ID,
		anchor_at: null,
		kind: 'come_back',
		client_id: ID,
		client_name: 'Ana Rojas',
		due_date: '2026-10-31',
		late: false,
		text: 'Hola',
		phone_digits: null,
		wa_link: null,
		mode: 'message',
		prompt: null,
		// 1 Oct, 10:00 in Costa Rica.
		source_starts_at: '2026-10-01T16:00:00Z',
		source_services: ['Uñas'],
		general: false,
		offset_days: 30,
		shown_until: '2026-12-30',
		message_number: 1,
		next_due_date: null,
		...over
	};
}

describe('dueLabel', () => {
	it('a specific item: visit, services and the days left', () => {
		expect(dueLabel(item({}), TODAY, ZONE, true)).toEqual({
			text: 'Cita 1 oct · Uñas · quedan 60',
			warning: false
		});
	});

	it('a general item names its rule; the 2nd message is numbered', () => {
		const label = dueLabel(item({ general: true, message_number: 2 }), TODAY, ZONE, true);
		expect(label.text).toBe('Cita 1 oct · Uñas · General 30 días · 2º mensaje · quedan 60');
	});

	it('two service names, then +n for the rest', () => {
		const label = dueLabel(
			item({ source_services: ['Uñas', 'Cejas', 'Pedicure', 'Relleno'] }),
			TODAY,
			ZONE,
			true
		);
		expect(label.text).toBe('Cita 1 oct · Uñas, Cejas +2 · quedan 60');
	});

	it('quedan 14 the day before the next general rule is due', () => {
		expect(dueLabel(item({ shown_until: '2026-11-14' }), TODAY, ZONE, true).text).toMatch(
			/quedan 14$/
		);
	});

	it('a week or less left is a warning', () => {
		expect(dueLabel(item({ shown_until: '2026-11-07' }), TODAY, ZONE, true)).toEqual({
			text: 'Cita 1 oct · Uñas · quedan 7',
			warning: true
		});
	});

	it('one day left, then the last day', () => {
		expect(dueLabel(item({ shown_until: '2026-11-01' }), TODAY, ZONE, true).text).toMatch(
			/queda 1 día$/
		);
		expect(dueLabel(item({ shown_until: TODAY }), TODAY, ZONE, true).text).toMatch(/Último día$/);
	});

	it('offline: no days left and no warning', () => {
		expect(dueLabel(item({ shown_until: TODAY }), TODAY, ZONE, false)).toEqual({
			text: 'Cita 1 oct · Uñas',
			warning: false
		});
	});

	it('a "before" item shows the visit time and never the days left', () => {
		const before = item({
			kind: 'before_appointment',
			source_starts_at: '2026-11-02T15:30:00Z',
			shown_until: '2026-11-02'
		});
		expect(dueLabel(before, TODAY, ZONE, true)).toEqual({
			text: 'Cita 2 nov 09:30 · Uñas',
			warning: false
		});
	});
});

describe('nextReminderText', () => {
	const next = { due_date: '2026-10-31', source_services: ['Uñas'], general: true };

	it.each([
		['a future day', '2026-10-20', 'Próximo mensaje: 31 oct (Uñas)'],
		['today', '2026-10-31', 'Mensaje para hoy (Uñas)'],
		['a past day', '2026-11-05', 'Mensaje atrasado desde 31 oct (Uñas)']
	])('says when the message is due on %s', (_, today, want) => {
		expect(nextReminderText(next, today)).toBe(want);
	});

	it('joins services as the row label does', () => {
		const three = { ...next, source_services: ['Uñas', 'Cejas', 'Pestañas'] };
		expect(nextReminderText(three, '2026-10-20')).toBe('Próximo mensaje: 31 oct (Uñas, Cejas +1)');
	});

	it('leaves the parentheses out for a visit with no services', () => {
		expect(nextReminderText({ ...next, source_services: [] }, '2026-10-31')).toBe(
			'Mensaje para hoy'
		);
	});

	it('says nothing is planned for none', () => {
		expect(nextReminderText(null, TODAY)).toBe('Sin mensajes pendientes');
	});
});
