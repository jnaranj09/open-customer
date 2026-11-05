// The reactive read model the screens use: the cached snapshot and meta, and whether the last
// refresh found the server off. Screens read it; they never read Dexie.
import { api } from '../api/app';
import type { Api } from '../api/client';
import type { ApiError, Result } from '../api/errors';
import type { Me } from '../api/schemas';
import { EMPTY_META, readAll, readMeta, writeMeta, type CacheSnapshot, type Meta } from './db';
import { systemNow } from '../time';
import { serialLatest } from '../serial';
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

	/** The refresh, with a failed cache write turned into a failed Result (code only in the log). */
	async function attempt(): Promise<Result<void>> {
		try {
			return await refresh(deps);
		} catch {
			console.warn('code=cache_write_failed');
			return { ok: false, error: { kind: 'cache_write_failed' } };
		}
	}

	/** Fetches `/bootstrap`, then reloads what changed. Every outcome is recorded: `offline` is true
	 *  only when the server did not answer, and `lastError` holds any failure (null after success). */
	async function runOnce(): Promise<Result<void>> {
		const result = await attempt();
		offline = !result.ok && result.error.kind === 'offline';
		lastError = result.ok ? null : result.error;
		await load();
		return result;
	}

	/** Two refreshes never overlap: a call made during one waits for it and then refreshes once more,
	 *  so an older answer never overwrites a newer one. */
	const run = serialLatest(runOnce);

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
		/** Keeps the user the server just named, so an offline open can show the cached app. */
		async rememberMe(me: Me): Promise<void> {
			await writeMeta({ me });
			meta = await readMeta();
		},
		/** Reads the cache only, never the server. */
		load,
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
