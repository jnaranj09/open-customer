// The reactive read model the screens use: the cached snapshot and meta, and whether the last
// refresh found the server off. Screens read it; they never read Dexie.
import { api } from '../api/app';
import type { Api } from '../api/client';
import type { ApiError, Result } from '../api/errors';
import { EMPTY_META, readAll, readMeta, type CacheSnapshot, type Meta } from './db';
import { systemNow } from '../time';
import { appBadge, refresh, type Badge } from './refresh';

type Deps = { api: Api; now: () => Date; badge: Badge };

export function createCache(deps: Deps) {
	let snapshot = $state<CacheSnapshot | null>(null);
	let meta = $state<Meta>(EMPTY_META);
	let offline = $state(false);
	let lastError = $state<ApiError | null>(null);

	async function load(): Promise<void> {
		snapshot = await readAll();
		meta = await readMeta();
	}

	/** Fetches `/bootstrap`, then reloads what changed. Every outcome is recorded: `offline` is true
	 *  only when the server did not answer, and `lastError` holds any failure (null after success). */
	async function run(): Promise<Result<void>> {
		const result = await refresh(deps);
		offline = !result.ok && result.error.kind === 'offline';
		lastError = result.ok ? null : result.error;
		await load();
		return result;
	}

	return {
		get snapshot(): CacheSnapshot | null {
			return snapshot;
		},
		get meta(): Meta {
			return meta;
		},
		get offline(): boolean {
			return offline;
		},
		/** Why the last refresh failed, or null. */
		get lastError(): ApiError | null {
			return lastError;
		},
		/** Reads the cache; with none (first open, or a bad row just cleared it) asks for a refresh. */
		async reload(): Promise<void> {
			await load();
			if (snapshot === null) {
				await run();
			}
		},
		refresh: run
	};
}

export const cache = createCache({ api, now: systemNow, badge: appBadge });
