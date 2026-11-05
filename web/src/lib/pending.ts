// The send confirmation (design doc "Sending flow"): "Enviar" stores it before WhatsApp opens, so
// iOS closing the app while she is in WhatsApp cannot lose the question. It is asked on any app
// open and cleared by "Sí", "Todavía no", logout, or when its item is gone from the due list.
import type { Api } from './api/client';
import type { Result } from './api/errors';
import type { DueItem } from './api/schemas';
import type { PendingSend } from './cache/db';
import { applyBadge, type Badge } from './cache/refresh';

/** What the confirmation needs from the cache store. */
type PendingStore = {
	setPending(pending: PendingSend): Promise<void>;
	clearPending(): Promise<void>;
};

/** What the log POSTs tell the cache store: the server has this item, hide it (it is stored, so
 *  a reload keeps it hidden) until a refresh no longer lists it. "Sí" also clears the confirmation
 *  in the same write. */
type LoggedStore = { markLogged(id: string, opts: { clearPending: boolean }): Promise<void> };

/** A due item that has a WhatsApp link, so "Enviar" can run on it. */
export type Sendable = DueItem & { wa_link: string };

export function canSend(item: DueItem): item is Sendable {
	return item.wa_link !== null;
}

/** Stores the confirmation, then opens the link. If the store fails, the link stays closed: she
 *  would send a message the app could not ask about. */
export async function startSend(
	item: Sendable,
	open: (url: string) => void,
	deps: { store: PendingStore; now: () => Date }
): Promise<Result<void>> {
	try {
		await deps.store.setPending({
			item_id: item.id,
			client_name: item.client_name,
			started_at: deps.now().toISOString()
		});
	} catch {
		console.warn('code=cache_write_failed');
		return { ok: false, error: { kind: 'cache_write_failed' } };
	}
	open(item.wa_link);
	return { ok: true, data: undefined };
}

/** Whether to ask now, drop the confirmation, or do nothing. The caller passes the cached due
 *  list only after the cache is loaded: an empty list that was never read would clear it. */
export function pendingToAsk(p: PendingSend | null, due: DueItem[]): 'ask' | 'clear' | 'none' {
	if (p === null) {
		return 'none';
	}
	return due.some((i) => i.id === p.item_id) ? 'ask' : 'clear';
}

type LogDeps = {
	api: Pick<Api, 'recordReminder'>;
	store: LoggedStore;
	badge: Badge;
	now: () => Date;
	due: DueItem[];
};

/** The log POST both "Sí" and "Omitir" make. On the server's answer it hides the item from the
 *  cached list until a refresh no longer lists it (and, for "Sí", clears the confirmation in the
 *  same write), then updates the badge to the items that are left. A failed POST changes nothing.
 *  Rejects when the phone cannot store the change. */
async function logItem(
	item: DueItem,
	what: { action: 'sent'; body: string; phone_digits: string | null } | { action: 'skipped' },
	deps: LogDeps
): Promise<Result<void>> {
	const posted = await deps.api.recordReminder({
		rule_id: item.rule_id,
		appointment_id: item.appointment_id,
		anchor_at: item.anchor_at,
		...what,
		acted_at: deps.now().toISOString()
	});
	if (!posted.ok) {
		return posted;
	}
	await deps.store.markLogged(item.id, { clearPending: what.action === 'sent' });
	await applyBadge(deps.badge, deps.due.filter((i) => i.id !== item.id).length);
	return { ok: true, data: undefined };
}

/** "Sí": logs the item as sent with the exact text; the stored hide and the cleared confirmation
 *  are one write. A failed POST keeps the confirmation so she can tap "Sí" again (a repeated POST
 *  is safe on the server). `due` is the shown list (`cache.shownDue`), for the badge. */
export function confirmSent(item: DueItem, deps: LogDeps): Promise<Result<void>> {
	return logItem(item, { action: 'sent', body: item.text, phone_digits: item.phone_digits }, deps);
}

/** "Todavía no": the item stays in the list and the question is dropped. Only this, "Sí", logout
 *  and an item gone from the due list clear the question: the sheet cannot be dismissed. */
export async function notYet(store: PendingStore): Promise<void> {
	await store.clearPending();
}

/** "Omitir" after its "Deshacer" window: logs the item as skipped, then refreshes the cache. When
 *  the phone cannot store the hide, the server has still logged the skip: the refresh still runs
 *  and the result is `cache_write_failed`, so she reads that the phone did not save, not a generic
 *  error. A failed POST changes nothing and does not refresh. */
export async function skipItem(
	item: DueItem,
	deps: LogDeps & { refresh: () => Promise<unknown> }
): Promise<Result<void>> {
	let result: Result<void>;
	try {
		result = await logItem(item, { action: 'skipped' }, deps);
	} catch {
		console.warn('code=cache_write_failed');
		result = { ok: false, error: { kind: 'cache_write_failed' } };
	}
	if (result.ok || result.error.kind === 'cache_write_failed') {
		await deps.refresh();
	}
	return result;
}
