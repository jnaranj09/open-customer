import { describe, expect, it } from 'vitest';
import type { Status } from './api/schemas';
import { ageText, bytesText, statusLines } from './status';

const ZONE = 'America/Costa_Rica';
const NOW = new Date('2026-11-06T15:00:00Z');

function status(over: Partial<Status> = {}): Status {
	return {
		version: 'v1.2.0',
		migration_version: 12,
		heartbeat_age_s: 90,
		backup: {
			checked_at: '2026-11-05T22:00:00Z',
			last_dump_at: '2026-11-05T21:00:00Z',
			last_copy_at: '2026-11-05T21:30:00Z',
			last_error: null,
			red: false
		},
		media_free_bytes: 12_300_000_000,
		push: {
			subscriptions: 2,
			last_push: { date: '2026-11-05', attempted: 2, delivered: 1 },
			red: false
		},
		...over
	};
}

const value = (lines: ReturnType<typeof statusLines>, label: string): string | undefined =>
	lines.find((line) => line.label === label)?.value;

describe('ageText', () => {
	it('counts seconds under a minute', () => {
		expect(ageText(45)).toBe('hace 45 s');
	});
	it('counts minutes under an hour', () => {
		expect(ageText(90)).toBe('hace 1 min');
	});
	it('counts hours under a day', () => {
		expect(ageText(7300)).toBe('hace 2 h');
	});
	it('counts days after that', () => {
		expect(ageText(200_000)).toBe('hace 2 d');
	});
});

describe('bytesText', () => {
	it('writes gigabytes with one decimal', () => {
		expect(bytesText(12_300_000_000)).toBe('12,3 GB');
	});
	it('writes megabytes below a gigabyte', () => {
		expect(bytesText(450_000_000)).toBe('450 MB');
	});
});

describe('statusLines', () => {
	it('shows the version and the migration', () => {
		const lines = statusLines(status(), ZONE, NOW);
		expect([value(lines, 'status_version'), value(lines, 'status_migration')]).toEqual([
			'v1.2.0',
			'12'
		]);
	});
	it('says the timer has not ticked yet', () => {
		expect(value(statusLines(status({ heartbeat_age_s: null }), ZONE, NOW), 'status_timer')).toBe(
			'Aún sin latido'
		);
	});
	it('writes the last backup copy in the business zone', () => {
		expect(value(statusLines(status(), ZONE, NOW), 'status_backup')).toBe('5/11 15:30');
	});
	it('marks the backup line red with the problem text', () => {
		const lines = statusLines(
			status({
				backup: {
					checked_at: null,
					last_dump_at: null,
					last_copy_at: null,
					last_error: null,
					red: true
				}
			}),
			ZONE,
			NOW
		);
		expect(lines.find((line) => line.label === 'status_backup')).toEqual({
			label: 'status_backup',
			value: 'Respaldo con problema',
			red: true
		});
	});
	it('writes the year of a backup from another year', () => {
		const lines = statusLines(status(), ZONE, new Date('2027-01-10T12:00:00Z'));
		expect(value(lines, 'status_backup')).toBe('5/11/2026 15:30');
	});
	it('says there is no backup yet when it is not red and has no copy', () => {
		const lines = statusLines(
			status({
				backup: {
					checked_at: null,
					last_dump_at: null,
					last_copy_at: null,
					last_error: null,
					red: false
				}
			}),
			ZONE,
			NOW
		);
		expect(value(lines, 'status_backup')).toBe('Aún sin respaldo');
	});
	it('says the free space cannot be read', () => {
		expect(value(statusLines(status({ media_free_bytes: null }), ZONE, NOW), 'status_space')).toBe(
			'No disponible'
		);
	});
	it('marks the push line red when the server says so', () => {
		const lines = statusLines(
			status({ push: { subscriptions: 0, last_push: null, red: true } }),
			ZONE,
			NOW
		);
		expect(lines.find((line) => line.label === 'status_push')).toEqual({
			label: 'status_push',
			value: 'Notificaciones sin entregar',
			red: true
		});
	});
	it('counts the devices when push is fine', () => {
		expect(value(statusLines(status(), ZONE, NOW), 'status_push')).toBe('2 dispositivos');
	});
	it('writes the last push result', () => {
		expect(value(statusLines(status(), ZONE, NOW), 'status_last_push')).toBe('5/11: 1 de 2');
	});
	it('says no push was tried yet', () => {
		const lines = statusLines(
			status({ push: { subscriptions: 1, last_push: null, red: false } }),
			ZONE,
			NOW
		);
		expect(value(lines, 'status_last_push')).toBe('Aún sin avisos');
	});
});
