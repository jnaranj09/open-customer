// The one place that calls `fetch` for the API and the one parse site for its answers
// (rules/boundaries.md). Every method returns a `Result`; nothing here throws at a caller.
import createClient, { type Middleware } from 'openapi-fetch';
import type { z } from 'zod';
import type { ApiError, ProblemName, Result } from './errors';
import type { components, paths } from './schema';
import {
	BootstrapSchema,
	MeSchema,
	PROBLEM_PREFIX,
	ProblemSchema,
	PushKeySchema,
	PushSubscriptionIdSchema,
	SetupStatusSchema
} from './schemas';

export type ApiOptions = {
	fetch: typeof fetch;
	/** The build version of this app; a different `X-App-Version` from the server calls `onNewVersion`. */
	version: string;
	/** Runs on a 401 `unauthorized`: clear the cache and go to the login screen. */
	onUnauthorized: () => Promise<void>;
	onNewVersion: () => void;
	/** Told the route path (never the body) of a response that failed its schema. */
	reportInvalid: (path: string) => void;
	/** The API root. Absolute in tests; the app uses the default. */
	baseUrl?: string;
};

type Raw = { data?: unknown; error?: unknown; response: Response };
type Schemas = components['schemas'];

/** Thrown by the response middleware for an answer that did not come from the app. */
class NotTheApp extends Error {}

export function createApi(opts: ApiOptions) {
	const client = createClient<paths>({ baseUrl: opts.baseUrl ?? '/api/v1', fetch: opts.fetch });

	// Runs before openapi-fetch reads the body. The app sets X-App-Version on every answer, so an
	// answer without it came from Cloudflare (502, 524, 530): the server is off.
	const versionCheck: Middleware = {
		onResponse({ response }) {
			const server = response.headers.get('X-App-Version');
			if (!server) {
				throw new NotTheApp('no X-App-Version');
			}
			if (server !== opts.version) {
				opts.onNewVersion();
			}
		}
	};
	client.use(versionCheck);

	const invalid = (path: string): ApiError => {
		opts.reportInvalid(path);
		return { kind: 'response_invalid', path };
	};

	async function failure(path: string, raw: Raw): Promise<ApiError> {
		const parsed = ProblemSchema.safeParse(raw.error);
		if (!parsed.success) {
			return invalid(path);
		}
		const problem = parsed.data;
		const type = problem.type.slice(PROBLEM_PREFIX.length) as ProblemName;
		if (type === 'unauthorized') {
			try {
				await opts.onUnauthorized();
			} catch {
				// The caller still gets `unauthorized`; a hook that threw must not reject the call.
				console.warn('code=on_unauthorized_failed');
			}
			return { kind: 'unauthorized' };
		}
		if (type === 'overlap') {
			return problem.appointments
				? { kind: 'overlap', overlapping: problem.appointments }
				: invalid(path);
		}
		const fields = Object.fromEntries((problem.errors ?? []).map((e) => [e.field, e.message]));
		return { kind: 'problem', status: problem.status, type, fields, problem };
	}

	/** Sends the request; gives back the answer, or the error for offline and for a non-2xx answer. */
	async function exchange(path: string, go: () => Promise<Raw>): Promise<Raw | ApiError> {
		let raw: Raw;
		try {
			raw = await go();
		} catch (e) {
			// A body that is not JSON arrives as a SyntaxError; anything else (network, NotTheApp) is offline.
			return e instanceof SyntaxError ? invalid(path) : { kind: 'offline' };
		}
		return raw.response.ok ? raw : failure(path, raw);
	}

	async function read<T>(
		path: string,
		schema: z.ZodType<T>,
		go: () => Promise<Raw>
	): Promise<Result<T>> {
		const got = await exchange(path, go);
		if ('kind' in got) {
			return { ok: false, error: got };
		}
		const parsed = schema.safeParse(got.data);
		return parsed.success ? { ok: true, data: parsed.data } : { ok: false, error: invalid(path) };
	}

	async function readEmpty(path: string, go: () => Promise<Raw>): Promise<Result<void>> {
		const got = await exchange(path, go);
		return 'kind' in got ? { ok: false, error: got } : { ok: true, data: undefined };
	}

	return {
		bootstrap: (range: { from: string; to: string }) =>
			read('/bootstrap', BootstrapSchema, () =>
				client.GET('/bootstrap', { params: { query: range } })
			),
		me: () => read('/me', MeSchema, () => client.GET('/me')),
		setupStatus: () => read('/setup/status', SetupStatusSchema, () => client.GET('/setup/status')),
		login: (body: Schemas['LoginInput']) =>
			read('/auth/login', MeSchema, () => client.POST('/auth/login', { body })),
		logout: () => readEmpty('/auth/logout', () => client.POST('/auth/logout')),
		pushKey: () => read('/push/key', PushKeySchema, () => client.GET('/push/key')),
		subscribePush: (body: Schemas['PushSubscriptionInput']) =>
			read('/push/subscriptions', PushSubscriptionIdSchema, () =>
				client.POST('/push/subscriptions', { body })
			),
		logClientError: (body: Schemas['ClientErrorInput']) =>
			readEmpty('/client-errors', () => client.POST('/client-errors', { body }))
	};
}

export type Api = ReturnType<typeof createApi>;
