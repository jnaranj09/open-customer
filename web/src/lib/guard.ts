// The redirects of the app shell (design doc item 71). Pure: the layout feeds it the path and what
// the server (or, offline, the cache) said about the session.
import type { Me } from './api/schemas';

export type Session =
	| { kind: 'online'; setupNeeded: boolean; me: Me | null }
	| { kind: 'offline'; cachedMe: Me | null };

export type GuardResult =
	| { kind: 'stay' }
	| { kind: 'redirect'; to: '/' | '/login' | '/setup' | '/cambiar-clave' }
	| { kind: 'offline_blank' };

/** The screens that work without a session. */
export const PUBLIC_PATHS: readonly string[] = ['/login', '/setup'];

const CHANGE_PASSWORD = '/cambiar-clave';

/** Where `path` should go given the session. Offline never redirects: with a cached `Me` the cached
 *  app shows, without one the layout shows the offline text. */
export function guard(path: string, s: Session): GuardResult {
	if (s.kind === 'offline') {
		return s.cachedMe === null ? { kind: 'offline_blank' } : { kind: 'stay' };
	}
	if (s.setupNeeded) {
		return path === '/setup' ? { kind: 'stay' } : { kind: 'redirect', to: '/setup' };
	}
	if (s.me === null) {
		return path === '/login' ? { kind: 'stay' } : { kind: 'redirect', to: '/login' };
	}
	if (s.me.must_change_password) {
		return path === CHANGE_PASSWORD ? { kind: 'stay' } : { kind: 'redirect', to: CHANGE_PASSWORD };
	}
	return PUBLIC_PATHS.includes(path) ? { kind: 'redirect', to: '/' } : { kind: 'stay' };
}

/** True for the pages that show the navigation; login, setup and the forced password change stand alone. */
export function hasShell(path: string): boolean {
	return !PUBLIC_PATHS.includes(path) && path !== CHANGE_PASSWORD;
}

/** True when the session answer can change without a reload: she logs in, finishes setup or sets
 *  the new password on a page the guard let her stay on, then moves on; a 401 sends her to a
 *  stand-alone page while the old answer still says she is signed in. The layout asks again. */
export function needsRecheck(s: Session | null, path: string): boolean {
	return (
		s === null ||
		!hasShell(path) ||
		(s.kind === 'online' && (s.setupNeeded || s.me === null || s.me.must_change_password))
	);
}

/** True when the cache should sync: she is signed in, or offline with a cached user. */
export function canSync(s: Session | null): boolean {
	if (s === null) {
		return false;
	}
	if (s.kind === 'offline') {
		return s.cachedMe !== null;
	}
	return !s.setupNeeded && s.me !== null && !s.me.must_change_password;
}
