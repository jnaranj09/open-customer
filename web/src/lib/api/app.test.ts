import { afterEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';
import { createAppApi, newVersion } from './app';

const VERSION = 'v1';
const PREFIX = 'urn:open-customer:problem:';
const headers = { 'Content-Type': 'application/json', 'X-App-Version': VERSION };

const problem = (name: string, status: number): Response =>
	new Response(JSON.stringify({ type: PREFIX + name, title: 'x', status }), {
		status,
		headers: { ...headers, 'Content-Type': 'application/problem+json' }
	});

const notAProblem = (): Response => new Response('{"oops":true}', { status: 500, headers });

function setup(answers: (() => Response)[]) {
	const fetchFn = vi.fn<typeof fetch>(() => Promise.reject(new TypeError('network down')));
	for (const answer of answers) {
		fetchFn.mockImplementationOnce(() => Promise.resolve(answer()));
	}
	const order: string[] = [];
	const api = createAppApi({
		fetch: fetchFn,
		version: VERSION,
		baseUrl: 'http://app.test/api/v1',
		clear: () => {
			order.push('clear');
			return Promise.resolve();
		},
		toLogin: () => {
			order.push('login');
			return Promise.resolve();
		}
	});
	return { api, fetchFn, order };
}

const flush = (): Promise<void> => new Promise((resolve) => setImmediate(resolve));

const bodyOf = (call: unknown[] | undefined): unknown => {
	const request = call?.[0];
	return request instanceof Request ? request.json() : undefined;
};

afterEach(() => {
	newVersion.set(false);
});

describe('the wired API client', () => {
	it('a 401 clears the cache, then goes to the login screen', async () => {
		const { api, order } = setup([() => problem('unauthorized', 401)]);
		await api.me();
		expect(order).toEqual(['clear', 'login']);
	});

	it('a different server version raises the update flag', async () => {
		const { api } = setup([
			() =>
				new Response(JSON.stringify({ needs_setup: false }), {
					headers: { ...headers, 'X-App-Version': 'v2' }
				})
		]);
		await api.setupStatus();
		expect(get(newVersion)).toBe(true);
	});

	it('reports an invalid answer with the route, a code and the app version', async () => {
		const { api, fetchFn } = setup([
			notAProblem,
			() => new Response(null, { status: 204, headers })
		]);
		await api.me();
		await flush();
		expect(fetchFn).toHaveBeenCalledTimes(2);
		expect(await bodyOf(fetchFn.mock.calls[1])).toEqual({
			message: 'response_invalid',
			route: '/me',
			app_version: VERSION
		});
	});

	it('does not report again when the invalid answer is to the report itself', async () => {
		const { api, fetchFn } = setup(Array.from({ length: 6 }, () => notAProblem));
		await api.me();
		await flush();
		expect(fetchFn).toHaveBeenCalledTimes(2);
	});
});
