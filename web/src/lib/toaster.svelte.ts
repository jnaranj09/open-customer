import { t, type MessageKey } from './i18n';

/** The message a screen shows in a `Toast`. `n` changes with each message, so the screen can key
 *  the toast on it and a repeat of the same text shows again. */
export type ToastState = { n: number; message: string };

export function createToaster() {
	let current = $state<ToastState | null>(null);
	let count = 0;
	return {
		get current(): ToastState | null {
			return current;
		},
		say(message: MessageKey): void {
			count += 1;
			current = { n: count, message: t(message) };
		}
	};
}
