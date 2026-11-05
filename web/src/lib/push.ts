// Web Push on the phone (design "Daily push", item 69). Push is requested only in the installed
// app and only from the "Activar notificaciones" tap (`activatePush`); `checkPush` runs on each
// open and never prompts.
import type { Api } from './api/client';
import type { Result } from './api/errors';
import { PushSubscriptionInputSchema } from './api/schemas';

export type PushState = 'unsupported' | 'not_installed' | 'needs_activation' | 'active';

/** The browser and the server behind push, so tests can stand in for both. */
export type PushDeps = {
	api: Pick<Api, 'pushKey' | 'subscribePush'>;
	/** The browser has service workers, push and notifications. */
	supported: boolean;
	/** The app runs from the Home Screen (display-mode standalone). */
	standalone: boolean;
	permission: () => NotificationPermission;
	requestPermission: () => Promise<NotificationPermission>;
	/** The service worker registration that owns the push manager. */
	registration: () => Promise<{ pushManager: PushManagerLike }>;
};

type PushManagerLike = Pick<PushManager, 'getSubscription' | 'subscribe'>;

const UNAVAILABLE: Result<never> = { ok: false, error: { kind: 'push_unavailable' } };

/** The VAPID key is base64url without padding; `subscribe` wants its bytes. */
function keyBytes(key: string): Uint8Array<ArrayBuffer> {
	const padded = key.replace(/-/g, '+').replace(/_/g, '/');
	const raw = atob(padded.padEnd(Math.ceil(padded.length / 4) * 4, '='));
	return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

/** Posts the subscription as the browser describes it. A JSON that is not a subscription is a
 *  failure of the browser, not of the network. */
async function post(deps: PushDeps, sub: PushSubscription): Promise<Result<void>> {
	const parsed = PushSubscriptionInputSchema.safeParse(sub.toJSON());
	if (!parsed.success) {
		console.warn('code=push_subscription_invalid');
		return UNAVAILABLE;
	}
	const sent = await deps.api.subscribePush(parsed.data);
	return sent.ok ? { ok: true, data: undefined } : sent;
}

/** Run on each open of the installed app. Re-posts an existing subscription, so one the server
 *  deleted comes back. Never asks for permission: iOS allows that only after a tap. */
export async function checkPush(deps: PushDeps): Promise<PushState> {
	// Standalone first: iOS Safari tabs have no PushManager, and "install it" is the right advice.
	if (!deps.standalone) {
		return 'not_installed';
	}
	if (!deps.supported) {
		return 'unsupported';
	}
	if (deps.permission() !== 'granted') {
		return 'needs_activation';
	}
	try {
		const sub = await (await deps.registration()).pushManager.getSubscription();
		if (!sub) {
			return 'needs_activation';
		}
		const posted = await post(deps, sub);
		if (!posted.ok && posted.error.kind === 'push_unavailable') {
			return 'needs_activation';
		}
		// A failed re-post (offline) keeps the subscription; the next open tries again.
		return 'active';
	} catch {
		console.warn('code=push_check_failed');
		return 'needs_activation';
	}
}

/** The "Activar notificaciones" tap: ask for permission, subscribe with the server's key, store the
 *  subscription. The permission prompt comes first, before any request, to stay inside the tap. */
export async function activatePush(deps: PushDeps): Promise<Result<void>> {
	if (!deps.standalone || !deps.supported) {
		return UNAVAILABLE;
	}
	try {
		const permission = deps.permission() === 'granted' ? 'granted' : await deps.requestPermission();
		if (permission !== 'granted') {
			return UNAVAILABLE;
		}
		const { pushManager } = await deps.registration();
		const existing = await pushManager.getSubscription();
		if (existing) {
			return await post(deps, existing);
		}
		const key = await deps.api.pushKey();
		if (!key.ok) {
			return key;
		}
		const sub = await pushManager.subscribe({
			userVisibleOnly: true,
			applicationServerKey: keyBytes(key.data.public_key)
		});
		return await post(deps, sub);
	} catch {
		console.warn('code=push_activate_failed');
		return UNAVAILABLE;
	}
}
