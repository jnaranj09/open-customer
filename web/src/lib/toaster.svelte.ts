import { t, type MessageKey } from './i18n';

/** The message the app shows in its one `Toast`. `n` changes with each message, so the host keys
 *  the toast on it and a repeat of the same text shows again. */
export type ToastState = {
	n: number;
	message: string;
	/** An undo button on the toast: its label and what it does. */
	action?: { label: string; run: () => void };
};

/** The app's one toast (release-1 bug L9: several screens each had their own, and two could
 *  overlap at the top). Every screen says its message here and the root layout shows it, so a new
 *  message replaces the one on screen. Exported for its test; screens use `toaster`. */
export function createToaster() {
	let current = $state<ToastState | null>(null);
	let count = 0;
	return {
		get current(): ToastState | null {
			return current;
		},
		/** Shows `message` (with `params`, and an action when given) in place of any toast on
		 *  screen; returns its `n` for `dismiss`. */
		say(
			message: MessageKey,
			action?: { label: MessageKey; run: () => void },
			params?: Record<string, string | number>
		): number {
			count += 1;
			current = {
				n: count,
				message: t(message, params),
				...(action === undefined ? {} : { action: { label: t(action.label), run: action.run } })
			};
			return count;
		},
		/** Takes toast `n` away, if it is still the one on screen (its action no longer applies). */
		dismiss(n: number): void {
			if (current?.n === n) {
				current = null;
			}
		}
	};
}

export const toaster = createToaster();
