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

/** "Sí": logs the item as sent with the exact text, then updates the badge and clears the
 *  confirmation. A failed POST changes nothing: the confirmation stays so she can tap "Sí" again
 *  (a repeated POST is safe on the server). `due` is the current list, for the badge. */
export async function confirmSent(
	p: PendingSend,
	item: DueItem,
	deps: {
		api: Pick<Api, 'recordReminder'>;
		store: PendingStore;
		badge: Badge;
		now: () => Date;
		due: DueItem[];
	}
): Promise<Result<void>> {
	const posted = await deps.api.recordReminder({
		rule_id: item.rule_id,
		appointment_id: item.appointment_id,
		anchor_at: item.anchor_at,
		action: 'sent',
		body: item.text,
		phone_digits: item.phone_digits,
		acted_at: deps.now().toISOString()
	});
	if (!posted.ok) {
		return posted;
	}
	await applyBadge(deps.badge, deps.due.filter((i) => i.id !== p.item_id).length);
	await deps.store.clearPending();
	return { ok: true, data: undefined };
}

/** "Todavía no": the item stays in the list and the question is dropped. */
export async function notYet(store: PendingStore): Promise<void> {
	await store.clearPending();
}

/** "Omitir" after its "Deshacer" window: logs the item as skipped, then updates the badge. The
 *  caller refreshes the cache after a success. `due` is the current list, for the badge. */
export async function skipItem(
	item: DueItem,
	deps: { api: Pick<Api, 'recordReminder'>; badge: Badge; now: () => Date; due: DueItem[] }
): Promise<Result<void>> {
	const posted = await deps.api.recordReminder({
		rule_id: item.rule_id,
		appointment_id: item.appointment_id,
		anchor_at: item.anchor_at,
		action: 'skipped',
		acted_at: deps.now().toISOString()
	});
	if (!posted.ok) {
		return posted;
	}
	await applyBadge(deps.badge, deps.due.filter((i) => i.id !== item.id).length);
	return { ok: true, data: undefined };
}
