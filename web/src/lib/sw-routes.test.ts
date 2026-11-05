import { describe, expect, it } from 'vitest';
import { pushCount, pushNotice, swAction } from './sw-routes';

const ORIGIN = 'https://app.test';
const precached: ReadonlySet<string> = new Set(['/index.html', '/_app/immutable/entry/start.js']);
const at = (path: string): URL => new URL(path, ORIGIN);

describe('swAction', () => {
	it('a navigation outside /api gets the shell', () => {
		expect(swAction(at('/clientas/abc'), 'navigate', precached)).toBe('shell');
	});

	it('a navigation to /api goes to the network', () => {
		expect(swAction(at('/api/v1/me'), 'navigate', precached)).toBe('network');
	});

	it('/api/v1/export with download goes to the network', () => {
		expect(swAction(at('/api/v1/export'), 'navigate', precached)).toBe('network');
	});

	it('an api call from the page goes to the network', () => {
		expect(swAction(at('/api/v1/bootstrap'), 'cors', precached)).toBe('network');
	});

	it('the manifest goes to the network', () => {
		expect(swAction(at('/manifest.webmanifest'), 'navigate', precached)).toBe('network');
	});

	it('the icons go to the network', () => {
		expect(swAction(at('/icons/512.png'), 'navigate', precached)).toBe('network');
	});

	it('the apple touch icon goes to the network', () => {
		expect(swAction(at('/apple-touch-icon.png'), 'navigate', precached)).toBe('network');
	});

	it('a precached asset is precache', () => {
		expect(swAction(at('/_app/immutable/entry/start.js'), 'cors', precached)).toBe('precache');
	});

	it('a file that is not precached and not a navigation goes to the network', () => {
		expect(swAction(at('/other.js'), 'cors', precached)).toBe('network');
	});
});

describe('pushCount', () => {
	it('reads n from the payload', () => {
		expect(pushCount({ title: 'Studio', body: 'Tienes 3', count: 3, url: '/enviar' })).toBe(3);
	});

	it.each([null, 'x', {}, { count: -1 }, { count: 1.5 }, { count: '3' }])(
		'rejects junk %j',
		(junk) => {
			expect(pushCount(junk)).toBeNull();
		}
	);
});

describe('pushNotice', () => {
	it('gives the title, body and count the server sent', () => {
		expect(
			pushNotice({ title: 'Studio', body: 'Tienes 1 mensaje', count: 1, url: '/enviar' })
		).toEqual({ title: 'Studio', body: 'Tienes 1 mensaje', count: 1 });
	});

	it('is null when the title is missing', () => {
		expect(pushNotice({ body: 'x', count: 1 })).toBeNull();
	});
});
