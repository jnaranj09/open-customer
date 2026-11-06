// The wired API client the screens use (design doc item 66): the real `fetch`, the build version
// from SvelteKit, and the reactions to a 401, a new server version and an invalid answer.
import { version } from '$app/env';
import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { writable } from 'svelte/store';
import { clearCache } from '../cache/db';
import type { ErrorReport } from '../report';
import { rememberFinishing, sessionStore } from '../setup';
import { createApi, type Api } from './client';

/** True once the server reported a build version other than this app's; the update prompt reads it. */
export const newVersion = writable(false);

export type AppApiEnv = {
	fetch: typeof fetch;
	version: string;
	/** Drops every cached record (the 401 path). */
	clear: () => Promise<void>;
	/** Shows the login screen. */
	toLogin: () => Promise<void>;
	/** The API root; absolute in tests, the app uses the default. */
	baseUrl?: string;
};

/** The one poster of client errors (design doc item 66): an invalid answer and the global error
 *  handlers both send through it. A failed post is dropped: nothing waits on it. */
function postErrorReport(
	client: Pick<Api, 'logClientError'>,
	report: ErrorReport
): Promise<unknown> {
	const { stack, ...rest } = report;
	return client.logClientError(stack === null ? rest : { ...rest, stack });
}

/** Builds the app's client from its outside edges; tests pass fakes. */
export function createAppApi(env: AppApiEnv): Api {
	const api: Api = createApi({
		fetch: env.fetch,
		version: env.version,
		...(env.baseUrl === undefined ? {} : { baseUrl: env.baseUrl }),
		onUnauthorized: async () => {
			// The login screen comes even when the cache cannot be cleared. The report is a console
			// line, not a client-error POST: the session is already dead, so that POST would 401 and
			// land here again.
			try {
				await env.clear();
			} catch {
				console.warn('code=cache_clear_failed');
			}
			await env.toLogin();
		},
		onNewVersion: () => {
			newVersion.set(true);
		},
		reportInvalid: (path) => {
			// The report is itself a request: an invalid answer to it must not report again.
			if (path === '/client-errors') {
				return;
			}
			void postErrorReport(api, {
				message: 'response_invalid',
				stack: null,
				route: path,
				app_version: env.version
			});
		}
	});
	return api;
}

export const api = createAppApi({
	fetch: (input, init) => fetch(input, init),
	version,
	// A lost session also drops the setup wizard's finishing step: the login screen comes next,
	// and a later sign-in must not reopen the wizard.
	clear: () => {
		rememberFinishing(sessionStore(), null);
		return clearCache();
	},
	toLogin: () => goto(resolve('/login'))
});

/** Sends a report of an uncaught error with the app's client. */
export function sendErrorReport(report: ErrorReport): void {
	void postErrorReport(api, report);
}
