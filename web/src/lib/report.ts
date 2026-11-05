// The global error report (design doc item 66 and the client-error endpoint): what the page sends
// to `POST /client-errors` for an uncaught error. The error's name and code, the route pattern and
// the stack only; never a message text or a response body, because those can hold a name or a phone.

const MESSAGE_MAX = 2000;
const STACK_MAX = 8000;
const ROUTE_MAX = 200;
const VERSION_MAX = 64;
/** The endpoint allows 10 a minute per session; the page stays under it. */
const SEND_LIMIT = 10;
const WINDOW_MS = 60_000;

export type ErrorReport = {
	message: string;
	stack: string | null;
	route: string;
	app_version: string;
};

/** The first `max` characters (the server counts runes, so a cut never splits one). */
function cut(text: string, max: number): string {
	return text.length <= max ? text : Array.from(text).slice(0, max).join('');
}

function codeOf(e: Error): string | null {
	const code: unknown = (e as { code?: unknown }).code;
	return typeof code === 'string' && code !== '' ? code : null;
}

/** The report for an uncaught error or rejection. `route` is the route pattern, not the URL. */
export function errorReport(e: unknown, route: string, version: string): ErrorReport {
	let message = 'non_error';
	let stack: string | null = null;
	if (e instanceof Error) {
		const code = codeOf(e);
		message = code === null ? e.name : `${e.name} ${code}`;
		// The stack's first line repeats the error text; the text stays out.
		if (e.stack !== undefined && e.stack !== '') {
			stack = e.message === '' ? e.stack : e.stack.split(e.message).join('');
		}
	}
	return {
		message: cut(message, MESSAGE_MAX),
		stack: stack === null ? null : cut(stack, STACK_MAX),
		route: cut(route === '' ? '/' : route, ROUTE_MAX),
		app_version: cut(version === '' ? 'unknown' : version, VERSION_MAX)
	};
}

/** A reporter that cannot spam: one report per distinct message per page load, and at most 10 a
 *  minute. A burst of window errors (or an error inside the report itself) sends a few lines. */
export function createReporter(
	send: (report: ErrorReport) => void,
	now: () => number
): (e: unknown, route: string, version: string) => void {
	const seen = new Set<string>();
	let sentAt: number[] = [];
	return (e, route, version) => {
		const report = errorReport(e, route, version);
		const at = now();
		sentAt = sentAt.filter((t) => at - t < WINDOW_MS);
		if (seen.has(report.message) || sentAt.length >= SEND_LIMIT) {
			return;
		}
		seen.add(report.message);
		sentAt.push(at);
		send(report);
	};
}
