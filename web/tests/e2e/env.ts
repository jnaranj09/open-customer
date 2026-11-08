// The one place the e2e run reads its environment (design doc item 79). The Playwright config
// (webServer) and the setup spec (`setup-link`) both build the app's env here, so the two
// processes see the same database.

/** Not 8080: that host port is the owner's, and production keeps it. `appEnv` passes the same port
 *  to the app as `LISTEN_PORT`. */
const PORT = 18080;
export const BASE_URL = `http://localhost:${String(PORT)}`;
/** The business zone and the browser's: no DST, so "today" is one day for the test, the browser
 *  and the server. */
export const ZONE = 'America/Costa_Rica';
/** The owner the setup spec creates. Made-up test credentials, never a real account. */
export const OWNER = { username: 'duena', password: 'prueba-e2e-2026' };
/** The saved login of the setup project: a session token, git-ignored. */
export const AUTH_FILE = 'tests/e2e/.auth/owner.json';

/** `E2E_DATABASE_URL`, set by `make e2e` and by CI; throws when it is missing. */
export function databaseUrl(): string {
	const url = process.env['E2E_DATABASE_URL'];
	if (url === undefined || url === '') {
		throw new Error('E2E_DATABASE_URL is not set');
	}
	return url;
}

/** The app's variables for `serve` and the admin commands: `DB_*` split out of
 *  `E2E_DATABASE_URL`, plain-http localhost, and `mediaDir` for the photos. */
export function appEnv(mediaDir: string): Record<string, string> {
	const db = new URL(databaseUrl());
	return {
		DB_HOST: db.hostname,
		DB_PORT: db.port === '' ? '5432' : db.port,
		DB_USER: decodeURIComponent(db.username),
		DB_PASSWORD: decodeURIComponent(db.password),
		DB_NAME: db.pathname.slice(1),
		DB_SSLMODE: 'disable',
		PUBLIC_URL: BASE_URL,
		LISTEN_PORT: String(PORT),
		VAPID_SUBJECT: 'mailto:e2e@localhost',
		MEDIA_DIR: mediaDir
	};
}
