import type { Result } from './api/errors';

export type LogoutDeps = {
	logout: () => Promise<Result<void>>;
	/** Deletes the whole local database (`clearCache`). */
	clear: () => Promise<void>;
	toLogin: () => Promise<void>;
};

/** Ends the session (design doc "Logout and lost session"): the server first, so a failed call
 *  leaves her signed in with her data; then the local database; then the login screen. A cache
 *  that cannot be cleared is logged by code and does not keep her on the page. */
export async function logOut(deps: LogoutDeps): Promise<Result<void>> {
	const result = await deps.logout();
	if (!result.ok) {
		return result;
	}
	try {
		await deps.clear();
	} catch {
		console.warn('code=cache_clear_failed');
	}
	await deps.toLogin();
	return result;
}
