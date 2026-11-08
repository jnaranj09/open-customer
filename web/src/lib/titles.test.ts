import { readdirSync } from 'node:fs';
import { dirname, join, sep } from 'node:path';
import { describe, expect, it } from 'vitest';
import { pageTitle } from './titles';

const ROUTES = join(import.meta.dirname, '../routes');

/** The SvelteKit route id of every `+page.svelte` on disk. */
function routeIds(): string[] {
	return readdirSync(ROUTES, { recursive: true, encoding: 'utf8' })
		.filter((f) => f.endsWith('+page.svelte'))
		.map((f) => dirname(f).split(sep).join('/'))
		.map((dir) => (dir === '.' ? '/' : `/${dir}`));
}

describe('pageTitle', () => {
	it('gives Hoy its Spanish title', () => {
		expect(pageTitle('/')).toBe('Hoy');
	});

	it('names a client page without her name', () => {
		expect(pageTitle('/clientas/[id]')).toBe('Clienta');
	});

	it('gives Cotizar its own title', () => {
		expect(pageTitle('/servicios')).toBe('Cotizar');
	});

	it('gives the moved catalog the Servicios title', () => {
		expect(pageTitle('/servicios/catalogo')).toBe('Servicios');
	});

	it('falls back to the app name for a path with no route', () => {
		expect(pageTitle(null)).toBe('Agenda');
	});

	it('gives every route on disk its own title, not the fallback', () => {
		const ids = routeIds();
		expect(ids.length).toBeGreaterThan(10);
		expect(ids.filter((id) => pageTitle(id) === pageTitle(null))).toEqual([]);
	});
});
