import { describe, expect, it } from 'vitest';
import { NAV_ITEMS } from './components/navItems';
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

describe('the Servicios place', () => {
	const place = NAV_ITEMS.find((item) => item.label === 'nav_services');

	it('opens on Cotizar', () => {
		expect(place?.href).toBe('/servicios');
	});

	it('is active on /servicios', () => {
		expect(place !== undefined && isActivePath(place.href, '/servicios')).toBe(true);
	});

	it('is active on /servicios/catalogo', () => {
		expect(place !== undefined && isActivePath(place.href, '/servicios/catalogo')).toBe(true);
	});

	it('is not active on Ajustes', () => {
		expect(place !== undefined && isActivePath(place.href, '/ajustes/negocio')).toBe(false);
	});
});

describe('syncLine', () => {
	it('shows the time of the last sync in the business zone', () => {
		expect(syncLine('2026-11-04T18:42:00Z', 'UTC', false, '2026-11-04')).toBe('Datos de las 18:42');
	});

	it('shows the day too when the last sync was on another day', () => {
		expect(syncLine('2026-11-03T18:42:00Z', 'UTC', false, '2026-11-04')).toBe(
			'Datos del 3/11 a las 18:42'
		);
	});

	it('shows the year when the last sync was in another year', () => {
		expect(syncLine('2025-12-31T18:42:00Z', 'UTC', false, '2026-01-02')).toBe(
			'Datos del 31/12/2025 a las 18:42'
		);
	});

	it('reads the day in the business zone', () => {
		expect(syncLine('2026-11-05T02:00:00Z', 'America/Costa_Rica', false, '2026-11-04')).toBe(
			'Datos de las 20:00'
		);
	});

	it('says there is no data before the first sync', () => {
		expect(syncLine(null, 'UTC', false, '2026-11-04')).toBe('Sin datos todavía');
	});

	it('says the system is off when offline', () => {
		expect(syncLine('2026-11-04T18:42:00Z', 'UTC', true, '2026-11-04')).toBe('Sin conexión');
	});
});
