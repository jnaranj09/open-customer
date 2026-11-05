import { describe, expect, it } from 'vitest';
import { isActivePath, syncLine } from './nav';

describe('isActivePath', () => {
	it('matches the home item only on /', () => {
		expect([isActivePath('/', '/'), isActivePath('/', '/clientas')]).toEqual([true, false]);
	});

	it('matches an item on its own path', () => {
		expect(isActivePath('/clientas', '/clientas')).toBe(true);
	});

	it('matches an item on a path below it', () => {
		expect(isActivePath('/clientas', '/clientas/abc')).toBe(true);
	});

	it('does not match a path that only shares a prefix', () => {
		expect(isActivePath('/ajustes', '/ajustes-viejos')).toBe(false);
	});
});

describe('syncLine', () => {
	it('shows the time of the last sync in the business zone', () => {
		expect(syncLine('2026-11-04T18:42:00Z', 'UTC', false)).toBe('Datos de las 18:42');
	});

	it('says there is no data before the first sync', () => {
		expect(syncLine(null, 'UTC', false)).toBe('Sin datos todavía');
	});

	it('says the system is off when offline', () => {
		expect(syncLine('2026-11-04T18:42:00Z', 'UTC', true)).toBe('Sin conexión con el sistema');
	});
});
