// Copy to the clipboard with a short "Copiado" (the quote text, a quick reply). The components keep
// their own `copied` state; this file holds the clipboard call and the timer.

/** How long "Copiado" shows after a copy. */
export const COPIED_MS = 2000;

/** Writes `text` to the clipboard, then calls `done`, or `failed` when the clipboard is missing (outside
 *  a secure context) or refuses. Call it inside the tap, with nothing awaited before it: iOS allows
 *  the write only then. A failure logs `code=clipboard_failed`. */
export function copyText(text: string, done: () => void, failed: () => void): void {
	const fail = (): void => {
		console.warn('code=clipboard_failed');
		failed();
	};
	// Missing outside a secure context, which the DOM types do not show.
	const clipboard = navigator.clipboard as Clipboard | undefined;
	if (clipboard === undefined) {
		fail();
		return;
	}
	try {
		void clipboard.writeText(text).then(done, fail);
	} catch {
		fail();
	}
}

/** A "Copiado" flag: `show` sets it and clears it `COPIED_MS` after the last copy; `stop` drops the
 *  timer (on destroy). */
export function copiedFlag(set: (on: boolean) => void): { show: () => void; stop: () => void } {
	let timer: ReturnType<typeof setTimeout> | undefined;
	return {
		show: () => {
			set(true);
			clearTimeout(timer);
			timer = setTimeout(() => {
				set(false);
			}, COPIED_MS);
		},
		stop: () => {
			clearTimeout(timer);
		}
	};
}
