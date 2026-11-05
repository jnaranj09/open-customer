import { describe, expect, it } from 'vitest';
import { canSync, guard, hasShell, needsRecheck, PUBLIC_PATHS, type Session } from './guard';

const ME = {
	id: '3f2b8c1e-5d4a-4e6b-9c7d-1a2b3c4d5e6f',
	username: 'ana',
	must_change_password: false
};
const online = (over: Partial<Extract<Session, { kind: 'online' }>> = {}): Session => ({
	kind: 'online',
	setupNeeded: false,
	me: ME,
	...over
});

describe('guard, online', () => {
	it('without a session redirects to /login', () => {
		expect(guard('/clientas', online({ me: null }))).toEqual({ kind: 'redirect', to: '/login' });
	});

	it('with setup needed redirects to /setup', () => {
		expect(guard('/', online({ setupNeeded: true, me: null }))).toEqual({
			kind: 'redirect',
			to: '/setup'
		});
	});

	it('with setup needed redirects /login to /setup', () => {
		expect(guard('/login', online({ setupNeeded: true, me: null }))).toEqual({
			kind: 'redirect',
			to: '/setup'
		});
	});

	it('stays on /setup while setup is needed', () => {
		expect(guard('/setup', online({ setupNeeded: true, me: null }))).toEqual({ kind: 'stay' });
	});

	it('sends /setup to /login once setup is closed and there is no session', () => {
		expect(guard('/setup', online({ me: null }))).toEqual({ kind: 'redirect', to: '/login' });
	});

	it('stays on /login without a session', () => {
		expect(guard('/login', online({ me: null }))).toEqual({ kind: 'stay' });
	});

	it('must_change_password redirects to /cambiar-clave', () => {
		expect(guard('/', online({ me: { ...ME, must_change_password: true } }))).toEqual({
			kind: 'redirect',
			to: '/cambiar-clave'
		});
	});

	it('stays on /cambiar-clave when the password must change', () => {
		expect(guard('/cambiar-clave', online({ me: { ...ME, must_change_password: true } }))).toEqual({
			kind: 'stay'
		});
	});

	it('sends /login to / while logged in', () => {
		expect(guard('/login', online())).toEqual({ kind: 'redirect', to: '/' });
	});

	it('sends /setup to / while logged in', () => {
		expect(guard('/setup', online())).toEqual({ kind: 'redirect', to: '/' });
	});

	it('stays on an app page while logged in', () => {
		expect(guard('/clientas/abc', online())).toEqual({ kind: 'stay' });
	});
});

describe('guard, offline', () => {
	it('stays with a cached Me', () => {
		expect(guard('/clientas', { kind: 'offline', cachedMe: ME })).toEqual({ kind: 'stay' });
	});

	it('is offline_blank without a cached Me', () => {
		expect(guard('/', { kind: 'offline', cachedMe: null })).toEqual({ kind: 'offline_blank' });
	});

	it.each(['/', '/login', '/setup', '/cambiar-clave', '/enviar'])('never redirects %s', (path) => {
		expect(guard(path, { kind: 'offline', cachedMe: ME }).kind).not.toBe('redirect');
		expect(guard(path, { kind: 'offline', cachedMe: null }).kind).not.toBe('redirect');
	});
});

describe('hasShell', () => {
	it('is false for the public paths and the forced password change', () => {
		expect([...PUBLIC_PATHS, '/cambiar-clave'].map(hasShell)).toEqual([false, false, false]);
	});

	it('is true for an app page', () => {
		expect(hasShell('/clientas')).toBe(true);
	});
});

describe('needsRecheck', () => {
	it('is true before the first answer', () => {
		expect(needsRecheck(null, '/')).toBe(true);
	});

	it.each([
		['no session', online({ me: null })],
		['setup needed', online({ setupNeeded: true, me: null })],
		['a forced password change', online({ me: { ...ME, must_change_password: true } })]
	])('is true with %s', (_name, s) => {
		expect(needsRecheck(s, '/')).toBe(true);
	});

	it('is true on a stand-alone page even for a settled session: a 401 may have sent her there', () => {
		expect(needsRecheck(online(), '/login')).toBe(true);
	});

	it('is false for a settled session on an app page', () => {
		expect(needsRecheck(online(), '/clientas')).toBe(false);
	});

	it('is false offline on an app page', () => {
		expect(needsRecheck({ kind: 'offline', cachedMe: ME }, '/')).toBe(false);
	});
});

describe('canSync', () => {
	it('is false before the first answer', () => {
		expect(canSync(null)).toBe(false);
	});

	it('is true when signed in', () => {
		expect(canSync(online())).toBe(true);
	});

	it.each([
		['no session', online({ me: null })],
		['setup needed', online({ setupNeeded: true, me: null })],
		['a forced password change', online({ me: { ...ME, must_change_password: true } })],
		['offline without a cached user', { kind: 'offline', cachedMe: null } satisfies Session]
	])('is false with %s', (_name, s) => {
		expect(canSync(s)).toBe(false);
	});

	it('is true offline with a cached user', () => {
		expect(canSync({ kind: 'offline', cachedMe: ME })).toBe(true);
	});
});
