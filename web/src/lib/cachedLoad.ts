import type { ApiError, Result } from './api/errors';

/** Shows the cached copy at once, then the server's. A server that is off keeps the cached copy
 *  (and is not an error); with no copy, every failure is. Gives the error to show, `null` for
 *  none, or `undefined` when `current()` says the screen moved on to another record. */
export async function showThenFetch<T>(
	cached: T | undefined,
	get: () => Promise<Result<T>>,
	current: () => boolean,
	show: (value: T | null) => void
): Promise<ApiError | null | undefined> {
	show(cached ?? null);
	const result = await get();
	if (!current()) {
		return undefined;
	}
	if (result.ok) {
		show(result.data);
		return null;
	}
	return result.error.kind !== 'offline' || cached === undefined ? result.error : null;
}
