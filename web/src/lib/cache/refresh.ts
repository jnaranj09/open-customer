// The refresh of the phone cache (design doc item 67 and "Daily push"): `/bootstrap` for the
// window around the business day, every table replaced in one transaction, then the app badge.
import type { Api } from '../api/client';
import type { Result } from '../api/errors';
import { addDays, todayIn } from '../time';
import { readSettings, writeSnapshot } from './db';

/** The app icon badge on the Home Screen. */
export type Badge = { set(n: number): Promise<void>; clear(): Promise<void> };

/** A refresh on focus only when the last one is older than this. */
const FOCUS_MIN_AGE_MS = 30_000;

/** The `/bootstrap` window: 60 days back to 120 days ahead of `today` (`YYYY-MM-DD`). */
export function bootstrapWindow(today: string): { from: string; to: string } {
	return { from: addDays(today, -60), to: addDays(today, 120) };
}

/** True when the last refresh (epoch ms; null for none) is more than 30 s old. */
export function shouldRefreshOnFocus(lastAt: number | null, now: number): boolean {
	return lastAt === null || now - lastAt > FOCUS_MIN_AGE_MS;
}

/** The real badge: `navigator.setAppBadge` where the browser has it, nothing elsewhere. */
export const appBadge: Badge = {
	async set(n: number): Promise<void> {
		await badgeCall(() => navigator.setAppBadge(n));
	},
	async clear(): Promise<void> {
		await badgeCall(() => navigator.clearAppBadge());
	}
};

async function badgeCall(call: () => Promise<void>): Promise<void> {
	if (typeof navigator === 'undefined' || !('setAppBadge' in navigator)) {
		return;
	}
	try {
		await call();
	} catch {
		console.warn('code=badge_failed');
	}
}

/** Fetches `/bootstrap`, replaces the cache and sets the badge to the due count. The window uses
 *  the business zone from the cached settings, or the browser zone before the first refresh. A
 *  failed call changes nothing. A failed write rejects: the old data is still in place. */
export async function refresh(deps: {
	api: Api;
	now: () => Date;
	badge: Badge;
}): Promise<Result<void>> {
	const now = deps.now();
	const settings = await readSettings();
	const zone = settings?.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone;
	const answer = await deps.api.bootstrap(bootstrapWindow(todayIn(zone, now)));
	if (!answer.ok) {
		return answer;
	}
	await writeSnapshot(answer.data, now.toISOString());
	const due = answer.data.due.length;
	await (due > 0 ? deps.badge.set(due) : deps.badge.clear());
	return { ok: true, data: undefined };
}
