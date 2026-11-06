import { describe, expect, it, vi } from 'vitest';
import {
	openFromTap,
	pushCount,
	pushNotice,
	pushShown,
	swAction,
	type TapWindow
} from './sw-routes';

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

describe('pushShown', () => {
	const FALLBACK = {
		title: 'Mensajes para enviar',
		body: 'Abre la app para ver los mensajes de hoy',
		badge: null,
		valid: false
	};

	it('shows the title, body and count the server sent', () => {
		expect(
			pushShown(JSON.stringify({ title: 'Estudio Ana', body: 'Tienes 2 mensajes', count: 2 }))
		).toEqual({ title: 'Estudio Ana', body: 'Tienes 2 mensajes', badge: 2, valid: true });
	});

	it('a push with no payload still shows the fallback text', () => {
		expect(pushShown(null)).toEqual(FALLBACK);
	});

	it('a payload that is not JSON still shows the fallback text', () => {
		expect(pushShown('{not json')).toEqual(FALLBACK);
	});

	it('a payload of another shape still shows the fallback text', () => {
		expect(pushShown(JSON.stringify({ message: 'x' }))).toEqual(FALLBACK);
	});
});

/** A window whose `navigate` answers with `navigate`; it records the paths it was sent to. */
function tapWindow(navigate: (url: string) => Promise<unknown>): TapWindow & { sent: string[] } {
	const sent: string[] = [];
	return {
		sent,
		focus: () =>
			Promise.resolve({
				navigate: (url: string) => {
					sent.push(url);
					return navigate(url);
				}
			})
	};
}

describe('openFromTap', () => {
	it('moves the open window to the path', async () => {
		const open = tapWindow(() => Promise.resolve(null));
		const openWindow = vi.fn(() => Promise.resolve(null));
		await openFromTap([open], openWindow, '/enviar');
		expect([open.sent, openWindow.mock.calls.length]).toEqual([['/enviar'], 0]);
	});

	it('opens a new window when there is none', async () => {
		const openWindow = vi.fn(() => Promise.resolve(null));
		await openFromTap([], openWindow, '/enviar');
		expect(openWindow).toHaveBeenCalledWith('/enviar');
	});

	it('opens a new window when the open one cannot navigate (not controlled)', async () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => undefined);
		const open = tapWindow(() => Promise.reject(new TypeError('not controlled')));
		const openWindow = vi.fn(() => Promise.resolve(null));
		await openFromTap([open], openWindow, '/enviar');
		expect(openWindow).toHaveBeenCalledWith('/enviar');
		expect(warn).toHaveBeenCalledWith('code=notification_navigate_failed');
		warn.mockRestore();
	});
});
