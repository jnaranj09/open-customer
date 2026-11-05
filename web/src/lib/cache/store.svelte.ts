// The reactive read model the screens use: the cached snapshot and meta, and whether the last
// refresh found the server off. Screens read it; they never read Dexie.
import { api } from '../api/app';
import type { Api } from '../api/client';
import type { ApiError, Result } from '../api/errors';
import type { DueItem, Me } from '../api/schemas';
import { shownDue } from '../due';
import {
	EMPTY_META,
	onBadRow,
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
	// Due items the server has logged (a 201 or 200) that the cached list may still hold: the refresh
	// after the POST failed, or a refresh asked before the POST answered with the old list. A
	// refresh that succeeds drops only the ids its list no longer holds. Not optimistic: it follows
	// the answer.
	let logged = $state<string[]>([]);
	// Whether a session is live (the app shell says so); a bad cached row then asks `/bootstrap`.
	let syncing = false;
	let running = 0;

	// A cleared cache (401, bad row, logout) must not leave the old session's data on screen, nor
	// its user for an offline open to reuse.
	onCacheCleared(() => {
		snapshot = null;
		meta = EMPTY_META;
		offline = false;
		lastError = null;
		logged = [];
	});

	// A bad snapshot or meta row has just emptied the cache: ask the server again, so the screen does
	// not stay on "Cargando…". Not during a refresh: that one reads the cache again when it is done,
	// and a row that still fails then would only start another refresh.
	onBadRow(() => {
		if (syncing && running === 0) {
			void run().catch(() => {
				console.warn('code=cache_load_failed');
			});
		}
	});

	async function load(): Promise<void> {
		snapshot = await readAll();
		meta = await readMeta();
	}

	/** The refresh, with a failed cache write turned into a failed Result (code only in the log). */
	async function attempt(): Promise<Result<void>> {
		try {
			return await refresh(deps, () => logged);
		} catch {
			console.warn('code=cache_write_failed');
			return { ok: false, error: { kind: 'cache_write_failed' } };
		}
	}

	/** Fetches `/bootstrap`, then reloads what changed. Every outcome is recorded: `offline` is true
	 *  only when the server did not answer, and `lastError` holds any failure (null after success). */
	async function runOnce(): Promise<Result<void>> {
		running += 1;
		try {
			const result = await attempt();
			offline = !result.ok && result.error.kind === 'offline';
			lastError = result.ok ? null : result.error;
			await load();
			if (result.ok) {
				// A logged item never comes back on the server, so an id the new list still holds
				// is from a refresh asked before its log POST: it stays hidden.
				const due = snapshot?.due ?? [];
				logged = logged.filter((id) => due.some((i) => i.id === id));
			}
			return result;
		} finally {
			running -= 1;
		}
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
		/** Ids of due items already logged on the server that the cached list still holds. */
		get logged(): readonly string[] {
			return logged;
		},
		/** The due list every screen shows and counts (`shownDue`): the cached list without the
		 *  logged items and, offline, without a "before" item whose time has passed. */
		get shownDue(): DueItem[] {
			return shownDue(snapshot?.due ?? [], { now: deps.now(), offline, logged });
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
		/** The app shell says whether a session is live, so a bad cached row can ask for a refresh. */
		setSyncing(on: boolean): void {
			syncing = on;
		},
		/** Reads the cache only, never the server. */
		load,
		refresh: run
	};
}

export const cache = createCache({ api, now: systemNow, badge: appBadge });
