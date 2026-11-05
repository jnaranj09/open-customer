// The reactive read model the screens use: the cached snapshot and meta, and whether the last
// refresh found the server off. Screens read it; they never read Dexie.
import { api } from '../api/app';
import type { Api } from '../api/client';
import type { ApiError, Result } from '../api/errors';
import type { Me } from '../api/schemas';
import {
	EMPTY_META,
	onCacheCleared,
	readAll,
	readMeta,
	writeMeta,
	type CacheSnapshot,
	type Meta,
	type PendingSend
} from './db';
import { systemNow } from '../time';
import { serialLatest } from '../serial';
import { appBadge, refresh, type Badge } from './refresh';

type Deps = { api: Api; now: () => Date; badge: Badge };

export function createCache(deps: Deps) {
	let snapshot = $state<CacheSnapshot | null>(null);
	let meta = $state<Meta>(EMPTY_META);
	let offline = $state(false);
	let lastError = $state<ApiError | null>(null);
	// Due items the server has logged (a 201 or 200) that the cached list may still hold, because the
	// refresh after the POST failed. Hidden until a refresh succeeds. Not optimistic: it follows the answer.
	let logged = $state<string[]>([]);

	// A cleared cache (401, bad row, logout) must not leave the old session's data on screen, nor
	// its user for an offline open to reuse.
	onCacheCleared(() => {
		snapshot = null;
		meta = EMPTY_META;
		offline = false;
		lastError = null;
		logged = [];
	});

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
		if (result.ok) {
			logged = [];
		}
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
		/** Ids of due items already logged on the server, still in a cached list that has not refreshed. */
		get logged(): readonly string[] {
			return logged;
		},
		/** Notes that the server logged this due item. */
		markLogged(id: string): void {
			logged = [...logged, id];
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
		/** Stores the send confirmation (it survives a reload and a closed app). Rejects when the
		 *  write fails; the caller must not open WhatsApp then. */
		async setPending(pending: PendingSend): Promise<void> {
			await writeMeta({ pending });
			meta = await readMeta();
		},
		/** Clears the send confirmation. */
		async clearPending(): Promise<void> {
			await writeMeta({ pending: null });
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
